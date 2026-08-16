package graph

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// The type that every capability method takes.
const (
	contextPath = "context"
	contextName = "Context"
)

// capabilityMethods pairs each capability's method with the bit it sets and
// with whether the method returns an error. That result is the only way the
// three signatures differ.
var capabilityMethods = []struct {
	name         string
	bit          Capability
	returnsError bool
}{
	{"Start", Start, true},
	{"Quiesce", Quiesce, false},
	{"Stop", Stop, false},
}

// errorResult is the universe's error interface. A Start method declares it,
// and a Quiesce or Stop method must not.
var errorResult = types.Universe.Lookup("error").Type()

// Detect loads the package in dir and fills in the capabilities that each
// component's type declares. It reports a component whose type it cannot
// resolve, rather than leave the component with no capability: a component with
// none occupies no lifecycle level, and the lifecycle would run without it. It returns injectors of its own, and it leaves
// what it was given as it was.
//
// A capability is a fact about a type, so Detect is the one function here that
// needs the package to type-check.
//
// tags are the build tags that the run set. Google Wire received the same tags,
// and a provider that only one of the two loads can see builds no graph.
func Detect(dir string, tags []string, injectors []Injector) ([]Injector, []string, error) {
	caps, scope, err := capabilitiesIn(dir, tags)
	if err != nil {
		return nil, nil, err
	}

	filled := make([]Injector, 0, len(injectors))

	for _, inj := range injectors {
		components := make([]Component, 0, len(inj.Components))

		bound := caps[inj.Name]

		for _, c := range inj.Components {
			declared, ok := bound[c.Name]
			if !ok {
				return nil, nil, fmt.Errorf("injector %s: cannot resolve the type of %s", inj.Name, c.Name)
			}

			c.Capabilities = declared
			components = append(components, c)
		}

		inj.Components = components
		filled = append(filled, inj)
	}

	return filled, scope, nil
}

// capabilitiesIn type-checks the package in dir and reports what every value
// that a function binds declares, keyed by the function and then by the name.
func capabilitiesIn(dir string, tags []string) (caps map[string]map[string]Capability, scope []string, err error) {
	mode := packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo

	cfg := &packages.Config{Mode: mode, Dir: dir, BuildFlags: buildFlags(tags)}

	loaded, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", dir, err)
	}

	if len(loaded) == 0 {
		return nil, nil, fmt.Errorf("load %s: the directory holds no package", dir)
	}

	pkg := loaded[0]
	if len(pkg.Errors) > 0 {
		return nil, nil, fmt.Errorf("load %s: %w", dir, pkg.Errors[0])
	}

	caps = make(map[string]map[string]Capability)

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}

			caps[fn.Name.Name] = boundCapabilities(fn, pkg.TypesInfo)
		}
	}

	return caps, scopeNames(pkg), nil
}

// scopeNames returns every name that the package block declares. The lifecycle
// file shares that block, and Go forbids one name in both the file block and
// the package block, so an import of the lifecycle file takes none of them.
func scopeNames(pkg *packages.Package) []string {
	if pkg.Types == nil {
		return nil
	}

	scope := pkg.Types.Scope()

	return scope.Names()
}

// buildFlags returns the flags that set the run's own build tags on a load. It
// returns none when the run set no tag.
func buildFlags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	return []string{"-tags=" + strings.Join(tags, " ")}
}

// boundCapabilities reports what each value that fn binds declares.
func boundCapabilities(fn *ast.FuncDecl, info *types.Info) map[string]Capability {
	bound := make(map[string]Capability)

	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE {
			continue
		}

		for _, expr := range assign.Lhs {
			ident, ok := expr.(*ast.Ident)
			if !ok || ident.Name == blank {
				continue
			}

			obj := info.Defs[ident]
			if obj == nil || obj.Type() == nil {
				continue
			}

			bound[ident.Name] = capabilitiesOf(obj.Type())
		}
	}

	return bound
}

// capabilitiesOf reports which capability methods a type declares. It reads the
// type's method set, which is the one Go itself uses to decide an assignment: a
// pointer receiver's method belongs to the pointer type alone, an embedded
// type's methods are promoted, and an interface carries what it declares.
func capabilitiesOf(typ types.Type) Capability {
	methods := types.NewMethodSet(typ)

	caps := None

	for _, m := range capabilityMethods {
		if declares(methods, m.name, m.returnsError) {
			caps |= m.bit
		}
	}

	return caps
}

// declares reports whether the method set carries name with the signature that
// the capability requires: one context.Context parameter, and one error result
// when returnsError, none otherwise. A method that only shares the name is not
// a capability.
func declares(methods *types.MethodSet, name string, returnsError bool) bool {
	sel := methods.Lookup(nil, name)
	if sel == nil {
		return false
	}

	sig, ok := sel.Type().(*types.Signature)
	if !ok || !takesContext(sig.Params()) {
		return false
	}

	results := sig.Results()
	if !returnsError {
		return results.Len() == 0
	}

	return results.Len() == 1 && types.Identical(results.At(0).Type(), errorResult)
}

// takesContext reports whether params is exactly one context.Context.
func takesContext(params *types.Tuple) bool {
	if params.Len() != 1 {
		return false
	}

	typ := types.Unalias(params.At(0).Type())

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == contextPath && obj.Name() == contextName
}
