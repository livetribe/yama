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
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"l7e.io/yama/internal/generator/pkg"
)

// A directive comment starts with this prefix. The name of the directive
// follows it.
const directivePrefix = "//yama:"

// These are the Google Wire functions that the directive reader looks for.
const (
	newSetFunc = "NewSet"
	structFunc = "Struct"
)

// closeMethod is the method that a closer directive binds.
const closeMethod = "Close"

// A directiveKind is the directive that marks what builds a component.
type directiveKind int

const (
	noDirective directiveKind = iota
	closerDirective
	nocloseDirective
)

// directiveNames maps the name that follows the prefix to its kind.
var directiveNames = map[string]directiveKind{
	"closer":  closerDirective,
	"noclose": nocloseDirective,
}

// String returns the directive as the application writes it.
func (k directiveKind) String() string {
	for name, kind := range directiveNames {
		if kind == k {
			return directivePrefix + name
		}
	}

	return "no directive"
}

// A mark is the directive on one source, with where the source is. label names
// the source in a report.
type mark struct {
	kind  directiveKind
	at    token.Position
	label string
}

// readMarks reads the directive on each source that built a bound value. A
// source is a provider function or the struct type of a wire.Struct entry.
//
// readMarks also checks each directive comment in the target package. It
// reports a directive that is in no permitted position.
func readMarks(
	dir string,
	tags []string,
	target *packages.Package,
	bound map[string]map[string]*binding,
) (map[types.Object]mark, error) {
	marks, err := entryMarks(target)
	if err != nil {
		return nil, err
	}

	providers := providersIn(bound)

	decls, err := providerDecls(dir, tags, target, providers)
	if err != nil {
		return nil, err
	}

	for _, fn := range providers {
		label := providerLabel(fn, target.PkgPath)

		found, err := declMark(decls[fn], label)
		if err != nil {
			return nil, err
		}

		marks[fn] = found
	}

	return marks, nil
}

// providersIn returns each provider function that built a bound value, in a
// fixed order.
func providersIn(bound map[string]map[string]*binding) []*types.Func {
	seen := make(map[*types.Func]bool)

	var providers []*types.Func

	for _, values := range bound {
		for _, value := range values {
			fn, ok := value.source.(*types.Func)
			if !ok || seen[fn] {
				continue
			}

			seen[fn] = true
			providers = append(providers, fn)
		}
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].FullName() < providers[j].FullName()
	})

	return providers
}

// A providerDecl is the declaration of one provider, with the file set that
// gives its positions.
type providerDecl struct {
	decl *ast.FuncDecl
	fset *token.FileSet
}

// providerDecls finds the declaration of each provider. A provider in the
// target package comes from the target's own syntax. providerDecls loads every
// other provider's package by import path, with its source.
func providerDecls(
	dir string,
	tags []string,
	target *packages.Package,
	providers []*types.Func,
) (map[*types.Func]providerDecl, error) {
	sources := map[string]*packages.Package{target.PkgPath: target}

	var paths []string

	for _, fn := range providers {
		path := fn.Pkg().Path()
		if _, known := sources[path]; known {
			continue
		}

		sources[path] = nil
		paths = append(paths, path)
	}

	if len(paths) > 0 {
		mode := packages.NeedName | packages.NeedFiles | packages.NeedSyntax
		cfg := &packages.Config{Mode: mode, Dir: dir, BuildFlags: pkg.BuildFlags(tags)}

		loaded, err := packages.Load(cfg, paths...)
		if err != nil {
			return nil, fmt.Errorf("load the packages that declare providers: %w", err)
		}

		for _, p := range loaded {
			sources[p.PkgPath] = p
		}
	}

	decls := make(map[*types.Func]providerDecl, len(providers))

	for _, fn := range providers {
		source := sources[fn.Pkg().Path()]

		decl := funcDeclIn(source, fn.Name())
		if decl == nil {
			return nil, fmt.Errorf("cannot load the declaration of provider %s", fn.FullName())
		}

		decls[fn] = providerDecl{decl: decl, fset: source.Fset}
	}

	return decls, nil
}

// funcDeclIn returns the declaration of the function name in p. It returns nil
// when p is nil, and when p declares no such function.
func funcDeclIn(p *packages.Package, name string) *ast.FuncDecl {
	if p == nil {
		return nil
	}

	for _, file := range p.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == name {
				return fn
			}
		}
	}

	return nil
}

// declMark reads the directive in the doc comment of a provider's declaration.
// It reports a directive name that it does not know, and a provider that
// carries both directives.
func declMark(found providerDecl, label string) (mark, error) {
	at := found.fset.Position(found.decl.Pos())
	result := mark{at: at, label: label}

	if found.decl.Doc == nil {
		return result, nil
	}

	for _, c := range found.decl.Doc.List {
		name, ok := strings.CutPrefix(c.Text, directivePrefix)
		if !ok {
			continue
		}

		where := found.fset.Position(c.Pos())

		kind, known := directiveNames[name]
		if !known {
			return mark{}, fmt.Errorf("%s: unknown directive %s%s", where, directivePrefix, name)
		}

		if result.kind != noDirective && result.kind != kind {
			return mark{}, fmt.Errorf("%s: %s carries both %s and %s", where, label, result.kind, kind)
		}

		result.kind = kind
	}

	return result, nil
}

// providerLabel names a provider in a report. A provider outside the target
// package carries the name of its package.
func providerLabel(fn *types.Func, targetPath string) string {
	if fn.Pkg().Path() == targetPath {
		return "provider " + fn.Name()
	}

	return "provider " + fn.Pkg().Name() + "." + fn.Name()
}

// entryMarks reads the directive on each wire.Struct entry in the target
// package. The result is keyed by the name of the struct type that the entry
// names.
//
// entryMarks checks every directive comment in the target package. A directive
// is in the doc comment of a function declaration, or it occupies its own line
// immediately before a wire.Struct entry of a wire.NewSet call. entryMarks
// reports a directive in any other position. It also reports two wire.Struct
// entries that name one struct type and that do not carry the same directive.
func entryMarks(target *packages.Package) (map[types.Object]mark, error) {
	marks := make(map[types.Object]mark)

	for _, file := range target.Syntax {
		sets := newSetCalls(file, target.TypesInfo)

		marked, err := markedEntries(target, file, sets)
		if err != nil {
			return nil, err
		}

		for _, call := range sets {
			for _, arg := range call.Args {
				typ := structEntryType(arg, target.TypesInfo)
				if typ == nil {
					continue
				}

				at := target.Fset.Position(arg.Pos())
				found := mark{kind: marked[arg], at: at, label: "wire.Struct entry for " + typ.Name()}

				earlier, seen := marks[typ]
				if seen && earlier.kind != found.kind {
					return nil, fmt.Errorf("%s: %s carries %s, and the entry at %s carries %s",
						at, found.label, found.kind, earlier.at, earlier.kind)
				}

				marks[typ] = found
			}
		}
	}

	return marks, nil
}

// markedEntries returns the directive on each marked entry of sets. It reports
// each directive comment in file that marks no function declaration and no
// wire.Struct entry.
func markedEntries(
	target *packages.Package,
	file *ast.File,
	sets []*ast.CallExpr,
) (map[ast.Expr]directiveKind, error) {
	inDoc, err := docDirectives(target.Fset, file)
	if err != nil {
		return nil, err
	}

	marked := make(map[ast.Expr]directiveKind)

	for _, group := range file.Comments {
		for _, c := range group.List {
			name, ok := strings.CutPrefix(c.Text, directivePrefix)
			if !ok || inDoc[c] {
				continue
			}

			at := target.Fset.Position(c.Pos())

			kind, known := directiveNames[name]
			if !known {
				return nil, fmt.Errorf("%s: unknown directive %s%s", at, directivePrefix, name)
			}

			entry, err := markedEntry(target, c, sets)
			if err != nil {
				return nil, err
			}

			marked[entry] = kind
		}
	}

	return marked, nil
}

// docDirectives returns each directive comment that is in the doc comment of a
// function declaration. It reports a directive in the doc comment of a method.
func docDirectives(fset *token.FileSet, file *ast.File) (map[*ast.Comment]bool, error) {
	inDoc := make(map[*ast.Comment]bool)

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}

		for _, c := range fn.Doc.List {
			if !strings.HasPrefix(c.Text, directivePrefix) {
				continue
			}

			if fn.Recv != nil {
				at := fset.Position(c.Pos())

				return nil, fmt.Errorf("%s: %s is on the method %s, and a provider is never a method", at, c.Text, fn.Name.Name)
			}

			inDoc[c] = true
		}
	}

	return inDoc, nil
}

// markedEntry returns the wire.Struct entry that the directive comment c marks.
// The comment occupies its own line inside the argument list of a wire.NewSet
// call, and the entry starts on the next line. markedEntry reports a comment in
// any other position, and an entry that is not a wire.Struct entry.
func markedEntry(target *packages.Package, c *ast.Comment, sets []*ast.CallExpr) (ast.Expr, error) {
	fset := target.Fset
	at := fset.Position(c.Pos())

	call := enclosingSet(c, sets)
	if call == nil {
		return nil, fmt.Errorf("%s: %s marks no function declaration and no wire.Struct entry", at, c.Text)
	}

	entry, err := entryAfter(fset, c, call)
	if err != nil {
		return nil, err
	}

	if fn := funcOf(entry, target.TypesInfo); fn != nil {
		return nil, fmt.Errorf("%s: %s goes in the doc comment of the declaration of %s", at, c.Text, fn.Name())
	}

	if structEntryType(entry, target.TypesInfo) == nil {
		return nil, fmt.Errorf("%s: %s marks an entry that is not a wire.Struct entry", at, c.Text)
	}

	return entry, nil
}

// enclosingSet returns the innermost wire.NewSet call whose argument list
// holds c. It returns nil when no call does.
func enclosingSet(c *ast.Comment, sets []*ast.CallExpr) *ast.CallExpr {
	var innermost *ast.CallExpr

	for _, call := range sets {
		if c.Pos() < call.Lparen || c.End() > call.Rparen {
			continue
		}

		if innermost == nil || call.Lparen > innermost.Lparen {
			innermost = call
		}
	}

	return innermost
}

// entryAfter returns the argument of call that starts on the line after c. It
// reports a comment that shares its line with an argument or with the opening
// parenthesis, and a comment that no argument follows.
func entryAfter(fset *token.FileSet, c *ast.Comment, call *ast.CallExpr) (ast.Expr, error) {
	at := fset.Position(c.Pos())
	line := at.Line

	if fset.Position(call.Lparen).Line == line {
		return nil, fmt.Errorf("%s: %s must occupy its own line", at, c.Text)
	}

	for _, arg := range call.Args {
		if fset.Position(arg.End()).Line == line {
			return nil, fmt.Errorf("%s: %s must occupy its own line", at, c.Text)
		}
	}

	for _, arg := range call.Args {
		if fset.Position(arg.Pos()).Line == line+1 {
			return arg, nil
		}
	}

	return nil, fmt.Errorf("%s: no entry follows %s on the next line", at, c.Text)
}

// newSetCalls returns every call to wire.NewSet in file.
func newSetCalls(file *ast.File, info *types.Info) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && isWireCall(call, newSetFunc, info) {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

// isWireCall reports whether call is a call to the Google Wire function name.
func isWireCall(call *ast.CallExpr, name string, info *types.Info) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	qualifier, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	imported, ok := info.Uses[qualifier].(*types.PkgName)

	return ok && imported.Imported().Path() == pkg.WirePackagePath
}

// structEntryType returns the name of the struct type that a wire.Struct entry
// names. The entry's first argument is a pointer to that type. structEntryType
// returns nil when entry is not a wire.Struct entry.
func structEntryType(entry ast.Expr, info *types.Info) types.Object {
	call, ok := entry.(*ast.CallExpr)
	if !ok || !isWireCall(call, structFunc, info) || len(call.Args) == 0 {
		return nil
	}

	pointer, ok := info.TypeOf(call.Args[0]).(*types.Pointer)
	if !ok {
		return nil
	}

	return typeNameOf(pointer.Elem())
}

// applyMarks sets closer on each bound value that Yama wraps. It returns one
// warning for each source that builds a closer and carries no directive.
//
// applyMarks reports a marked source that builds no closer in an injector. It
// reports a closer directive on a provider that returns a cleanup, and on a
// value with a type that already declares Stop.
func applyMarks(bound map[string]map[string]*binding, marks map[types.Object]mark) ([]string, error) {
	warned := make(map[types.Object]string)

	for _, injector := range sortedKeys(bound) {
		var built []types.Object

		closable := make(map[types.Object]bool)
		values := bound[injector]

		for _, name := range sortedKeys(values) {
			value := values[name]
			if value.source == nil {
				continue
			}

			built = append(built, value.source)

			methods := types.NewMethodSet(value.typ)
			if !declaresClose(methods) {
				continue
			}

			closable[value.source] = true

			found := marks[value.source]
			if err := applyMark(value, found, warned); err != nil {
				return nil, err
			}
		}

		for _, source := range built {
			found := marks[source]
			if found.kind != noDirective && !closable[source] {
				return nil, fmt.Errorf("%s: %s marks %s, which builds no io.Closer in %s",
					found.at, found.kind, found.label, injector)
			}
		}
	}

	warnings := make([]string, 0, len(warned))
	for _, line := range warned {
		warnings = append(warnings, line)
	}

	sort.Strings(warnings)

	return warnings, nil
}

// applyMark applies the directive on the source of one value that declares
// Close. It records a warning for a source with no directive.
func applyMark(value *binding, found mark, warned map[types.Object]string) error {
	stops := value.capabilities.Has(Stop)

	switch found.kind {
	case closerDirective:
		if returnsCleanup(value.source) {
			return fmt.Errorf("%s: %s marks %s, which also returns a cleanup", found.at, found.kind, found.label)
		}

		if stops {
			return fmt.Errorf("%s: %s marks %s, which builds a type that already declares Stop",
				found.at, found.kind, found.label)
		}

		value.closer = true

	case nocloseDirective:

	case noDirective:
		if stops || returnsCleanup(value.source) {
			return nil
		}

		warned[value.source] = fmt.Sprintf("%s: %s builds an io.Closer that nothing closes: add %s or %s",
			found.at, found.label, closerDirective, nocloseDirective)
	}

	return nil
}

// declaresClose reports whether the method set carries Close with no parameter
// and one error result.
func declaresClose(methods *types.MethodSet) bool {
	sel := methods.Lookup(nil, closeMethod)
	if sel == nil {
		return false
	}

	sig, ok := sel.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 {
		return false
	}

	results := sig.Results()

	return results.Len() == 1 && types.Identical(results.At(0).Type(), errorResult)
}

// returnsCleanup reports whether source is a provider with a func() result.
func returnsCleanup(source types.Object) bool {
	fn, ok := source.(*types.Func)
	if !ok {
		return false
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	results := sig.Results()

	for i := range results.Len() {
		result, ok := results.At(i).Type().Underlying().(*types.Signature)
		if ok && result.Params().Len() == 0 && result.Results().Len() == 0 {
			return true
		}
	}

	return false
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
