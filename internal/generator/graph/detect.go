// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graph

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"

	"l7e.io/yama/internal/generator/pkg"
)

// This is the type that every capability method takes.
const (
	contextPath = "context"
	contextName = "Context"
)

// capabilityMethods pairs each capability's method with the bit that it sets,
// and with whether the method returns an error. That result is the only
// difference between the three signatures.
var capabilityMethods = []struct {
	name         string
	bit          Capability
	returnsError bool
}{
	{"Start", Start, true},
	{"Quiesce", Quiesce, false},
	{"Stop", Stop, false},
}

// errorResult is the universe's error interface. A Start method declares it. A
// Quiesce method or a Stop method must not declare it.
var errorResult = types.Universe.Lookup("error").Type()

// Detected is what Detect reads from a package.
type Detected struct {
	// Injectors are the injectors that Detect received, with each component's
	// capabilities and closer mark filled in.
	Injectors []Injector

	// Scope holds every name that the package block declares.
	Scope []string

	// Warnings holds one line for each unmarked closer, in a fixed order.
	Warnings []string
}

// Detect loads the package in dir. It fills in the capabilities that each
// component's type declares, and it marks each closer component. It reports a
// component with a type that it cannot resolve. It does not leave such a
// component with no capability. A component with no capability occupies no
// lifecycle level, and the lifecycle would then run without that component.
// Detect returns injectors of its own, and it makes no change to the injectors
// that it received.
//
// A capability is a fact about a type, so Detect is the one function here that
// needs the package to type-check.
//
// tags are the build tags that the run set. Google Wire received the same tags.
// A provider that only one of the two loads can see builds no graph.
func Detect(dir string, tags []string, injectors []Injector) (Detected, error) {
	target, err := loadTarget(dir, tags)
	if err != nil {
		return Detected{}, err
	}

	wanted := make(map[string]bool, len(injectors))
	for _, inj := range injectors {
		wanted[inj.Name] = true
	}

	bound := boundIn(target, wanted)

	marks, err := readMarks(dir, tags, target, bound)
	if err != nil {
		return Detected{}, err
	}

	warnings, err := applyMarks(bound, marks)
	if err != nil {
		return Detected{}, err
	}

	filled := make([]Injector, 0, len(injectors))

	for _, inj := range injectors {
		components := make([]Component, 0, len(inj.Components))

		values := bound[inj.Name]

		for _, c := range inj.Components {
			declared, ok := values[c.Name]
			if !ok {
				return Detected{}, fmt.Errorf("injector %s: cannot resolve the type of %s", inj.Name, c.Name)
			}

			c.Capabilities = declared.capabilities
			c.Closer = declared.closer
			components = append(components, c)
		}

		inj.Components = components
		filled = append(filled, inj)
	}

	scope := scopeNames(target)

	return Detected{Injectors: filled, Scope: scope, Warnings: warnings}, nil
}

// A binding is what one value that an injector binds declares. source is what
// built the value: a provider function, or the struct type of a struct
// literal. source is nil for any other value. closer is true when Yama wraps
// the value in a Stopper.
type binding struct {
	typ          types.Type
	capabilities Capability
	source       types.Object
	closer       bool
}

// loadTarget type-checks the package in dir.
func loadTarget(dir string, tags []string) (*packages.Package, error) {
	mode := packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo

	cfg := &packages.Config{Mode: mode, Dir: dir, BuildFlags: pkg.BuildFlags(tags)}

	loaded, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", dir, err)
	}

	if len(loaded) == 0 {
		return nil, fmt.Errorf("load %s: the directory holds no package", dir)
	}

	target := loaded[0]
	if len(target.Errors) > 0 {
		return nil, fmt.Errorf("load %s: %w", dir, target.Errors[0])
	}

	return target, nil
}

// boundIn reports what every value that a wanted function binds declares. The
// result is keyed by the function, and then by the name.
func boundIn(target *packages.Package, wanted map[string]bool) map[string]map[string]*binding {
	bound := make(map[string]map[string]*binding)

	for _, file := range target.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !wanted[fn.Name.Name] {
				continue
			}

			bound[fn.Name.Name] = boundValues(fn, target.TypesInfo)
		}
	}

	return bound
}

// scopeNames returns every name that the package block declares. The lifecycle
// file shares that block. Go forbids one name in both the file block and the
// package block. An import of the lifecycle file therefore takes none of these
// names.
func scopeNames(loaded *packages.Package) []string {
	if loaded.Types == nil {
		return nil
	}

	scope := loaded.Types.Scope()

	return scope.Names()
}

// boundValues reports what each value that fn binds declares. It records what
// built the first value of each statement.
func boundValues(fn *ast.FuncDecl, info *types.Info) map[string]*binding {
	bound := make(map[string]*binding)

	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE {
			continue
		}

		source := sourceOf(assign, info)

		for i, expr := range assign.Lhs {
			ident, ok := expr.(*ast.Ident)
			if !ok || ident.Name == blank {
				continue
			}

			obj := info.Defs[ident]
			if obj == nil || obj.Type() == nil {
				continue
			}

			value := &binding{typ: obj.Type(), capabilities: capabilitiesOf(obj.Type())}
			if i == 0 {
				value.source = source
			}

			bound[ident.Name] = value
		}
	}

	return bound
}

// sourceOf returns what built the value that assign binds. A call to a named
// function gives that function. A struct literal, or a pointer to one, gives
// the name of the struct type. sourceOf returns nil for any other right-hand
// side.
func sourceOf(assign *ast.AssignStmt, info *types.Info) types.Object {
	if len(assign.Rhs) != 1 {
		return nil
	}

	rhs := assign.Rhs[0]

	if call, ok := rhs.(*ast.CallExpr); ok {
		if fn := funcOf(call.Fun, info); fn != nil {
			return fn
		}

		return nil
	}

	if unary, ok := rhs.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		rhs = unary.X
	}

	lit, ok := rhs.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	litType := info.TypeOf(lit)

	return typeNameOf(litType)
}

// typeNameOf returns the name of a named type. It returns nil for any other
// type.
func typeNameOf(typ types.Type) types.Object {
	if typ == nil {
		return nil
	}

	named, ok := types.Unalias(typ).(*types.Named)
	if !ok {
		return nil
	}

	return named.Obj()
}

// funcOf returns the function that expr names. expr is an identifier or a
// qualified identifier. funcOf returns nil for any other expression, and for a
// name that is not a function.
func funcOf(expr ast.Expr, info *types.Info) *types.Func {
	var ident *ast.Ident

	switch e := expr.(type) {
	case *ast.Ident:
		ident = e
	case *ast.SelectorExpr:
		ident = e.Sel
	default:
		return nil
	}

	fn, _ := info.Uses[ident].(*types.Func)

	return fn
}

// capabilitiesOf reports which capability methods a type declares. It reads the
// type's method set. Go itself uses that same method set to decide an
// assignment. A pointer receiver's method belongs to the pointer type only. Go
// promotes an embedded type's methods. An interface carries what it declares.
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
// when returnsError is true. The method declares no result when returnsError is
// false. A method that only shares the name is not a capability.
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
