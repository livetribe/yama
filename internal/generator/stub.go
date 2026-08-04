// Copyright (c) 2026 the original author or authors.
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

package generator

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"

	"golang.org/x/tools/go/packages"
)

// yamaInjectTag gates a lifecycle stub file. Yama loads under it to see the
// stubs, and marks its own generated file `!yamainject` so the stub and the
// constructor it declares never compile together.
const yamaInjectTag = "yamainject"

// derivedPrefix begins the name of every injector Yama derives from a stub. An
// identifier carrying it is reserved: an application must not declare a Wire
// injector whose name starts with it.
const derivedPrefix = "yama_"

// wirePkgPath is Google Wire's public package, whose Build call a stub states
// its providers with, and wirePkgName is the name it declares.
const (
	wirePkgPath = "github.com/google/wire"
	wirePkgName = "wire"
)

// yamaPkgPath is Yama's public package, whose Option and Lifecycle a stub's
// signature names, and yamaPkgName is the name it declares. The path's last
// element is the major-version suffix rather than the package name, so a file
// importing it without an alias still refers to it as yamaPkgName.
const (
	yamaPkgPath = "l7e.io/yama/v2"
	yamaPkgName = "yama"
)

// optionTypeName is the variadic parameter type that may close a stub's
// parameter list.
const optionTypeName = "Option"

// defaultOptsName is the identifier the emitted constructor forwards its options
// under when the stub binds no usable name to the parameter.
const defaultOptsName = "opts"

// blankIdent is Go's blank identifier. A parameter bound to it carries no name
// that generated code can read.
const blankIdent = "_"

// lifecycleTypeName is the second result a stub declares.
const lifecycleTypeName = "Lifecycle"

// stubResults is the number of results a stub declares: the value the graph
// builds, the Lifecycle that orchestrates it, and the construction error.
const stubResults = 3

// ErrNoStubs reports that a package declares no lifecycle stub. It is not a
// failure; a sweep skips such a package rather than reporting it.
var ErrNoStubs = errors.New("yama: package has no lifecycle stub")

// StubError reports a lifecycle stub Yama cannot derive an injector from. It
// names the stub and the source position so the failure is a locatable
// build-time error.
type StubError struct {
	Stub string
	Pos  token.Position
	Msg  string
}

func (e *StubError) Error() string {
	return fmt.Sprintf("yama: lifecycle stub %s: %s: %s", e.Stub, e.Pos, e.Msg)
}

// Stub is one hand-authored lifecycle stub: the constructor name and signature
// an application declares, together with the wire.Build call stating the
// providers its graph is built from.
//
// Params holds the whole declared parameter list, the options parameter
// included. Results holds all three declared results. File is the stub's own
// file, whose imports name the packages the signature refers to.
//
// HasOpts reports whether the parameter list ends in a variadic yama.Option.
// A stub that ends in one gets a constructor that takes options. A stub that
// does not gets a constructor that takes none.
type Stub struct {
	Name    string
	Doc     *ast.CommentGroup
	Params  []Field
	Results []Field
	Build   *ast.CallExpr
	HasOpts bool

	File     *ast.File
	FuncDecl *ast.FuncDecl
}

// DerivedName is the injector Yama derives from this stub.
func (s *Stub) DerivedName() string {
	return derivedPrefix + s.Name
}

// GraphParams are the parameters the derived injector takes: the stub's own,
// without a trailing variadic Option parameter.
func (s *Stub) GraphParams() []Field {
	if !s.HasOpts {
		return s.Params
	}

	return s.Params[:len(s.Params)-1]
}

// OptsName is the identifier the emitted constructor forwards its options
// under: the name the stub bound the parameter to, or defaultOptsName when the
// stub bound no name or the blank identifier. Call it only when HasOpts is set.
func (s *Stub) OptsName() string {
	last := s.Params[len(s.Params)-1]
	if last.Name == "" || last.Name == blankIdent {
		return defaultOptsName
	}

	return last.Name
}

// ResultType is the type of the value the graph builds, taken from the stub's
// first declared result.
func (s *Stub) ResultType() ast.Expr {
	return s.Results[0].Type
}

// StubPackage is the lifecycle stubs one package declares, in a stable order:
// by file name, then by position within the file.
type StubPackage struct {
	Dir     string
	PkgName string
	PkgPath string
	Fset    *token.FileSet
	Stubs   []*Stub

	// ImportNames maps each package the stub file imports onto the name that
	// import's own package declares, resolved by the loader rather than guessed
	// from its path. An un-aliased import whose declared name differs from its
	// path's last element (any module path with a version suffix, for one) is
	// otherwise misnamed in the derived injector file.
	ImportNames map[string]string
}

// LoadStubs parses the lifecycle stubs declared in dir.
//
// The load runs under the yamainject tag, so it sees the stub file and not the
// lifecycle file Yama emits for the same constructors. A package that declares
// no stub yields ErrNoStubs.
//
// A stub file that fails to parse is reported as a load error rather than
// folded into ErrNoStubs: the two mean different things, a package with nothing
// to generate versus one Yama cannot read, and only the first is safe to skip
// in silence.
//
// The load type-checks. This load is the only one that sees the stubs beside
// the application's other files, so it is where the Go compiler catches a
// constructor whose name the package already declares. Yama cannot rename a
// constructor, because the application calls it, so reporting is the only
// answer available and the type checker gives it without a check of Yama's own.
func (g *Generator) LoadStubs(ctx context.Context, dir string) (*StubPackage, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Fset:       fset,
		BuildFlags: g.stubBuildFlags,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedImports | packages.NeedTypes,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("yama: loading lifecycle stubs in %s: %w", dir, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("yama: no package found in %s", dir)
	}

	pkg := pkgs[0]
	if loadErr := packageErrors(pkg); loadErr != nil {
		return nil, fmt.Errorf("yama: loading lifecycle stubs in %s: %w", dir, loadErr)
	}

	sp := &StubPackage{
		Dir:         dir,
		PkgName:     pkg.Name,
		PkgPath:     pkg.PkgPath,
		Fset:        fset,
		ImportNames: map[string]string{},
	}
	for path, imported := range pkg.Imports {
		sp.ImportNames[path] = imported.Name
	}

	for _, file := range pkg.Syntax {
		stubs, err := fileStubs(fset, file)
		if err != nil {
			return nil, err
		}
		sp.Stubs = append(sp.Stubs, stubs...)
	}

	if len(sp.Stubs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoStubs, dir)
	}

	sortStubs(fset, sp.Stubs)

	return sp, nil
}

// fileStubs extracts the stubs one file declares. A file that does not import
// Google Wire declares none, since a stub states its providers with wire.Build.
func fileStubs(fset *token.FileSet, file *ast.File) ([]*Stub, error) {
	wireName, ok := importName(file, wirePkgPath, wirePkgName)
	if !ok {
		return nil, nil
	}

	var stubs []*Stub
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}

		build, ok := stubBuildCall(fn, wireName)
		if !ok {
			continue
		}

		stub, err := newStub(fset, file, fn, build)
		if err != nil {
			return nil, err
		}
		stubs = append(stubs, stub)
	}

	return stubs, nil
}

// stubBuildCall returns the wire.Build call a stub's body consists of. A body
// of any other shape is not a stub, and is left to the application.
func stubBuildCall(fn *ast.FuncDecl, wireName string) (*ast.CallExpr, bool) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return nil, false
	}

	stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}

	panicCall, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	if id, isIdent := panicCall.Fun.(*ast.Ident); !isIdent || id.Name != "panic" {
		return nil, false
	}
	if len(panicCall.Args) != 1 {
		return nil, false
	}

	build, ok := panicCall.Args[0].(*ast.CallExpr)
	if !ok || !isSelector(build.Fun, wireName, "Build") {
		return nil, false
	}

	return build, true
}

// newStub validates one stub's signature and records it. A stub whose signature
// Yama cannot derive an injector from is a *StubError rather than a silently
// skipped declaration, since the application wrote it expecting a constructor.
func newStub(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, build *ast.CallExpr) (*Stub, error) {
	s := &Stub{
		Name:     fn.Name.Name,
		Doc:      fn.Doc,
		Params:   extractFields(fn.Type.Params),
		Results:  extractFields(fn.Type.Results),
		Build:    build,
		File:     file,
		FuncDecl: fn,
	}

	yamaName, ok := importName(file, yamaPkgPath, yamaPkgName)
	if !ok {
		yamaName = yamaPkgName
	}

	s.HasOpts = hasOptsDeclared(s, yamaName)

	if err := checkResults(fset, s, yamaName); err != nil {
		return nil, err
	}

	return s, nil
}

// hasOptsDeclared reports whether the stub's parameter list ends in a variadic
// Option parameter. The derived injector takes the stub's parameters without
// that one, and the emitted constructor forwards it to the builder.
func hasOptsDeclared(s *Stub, yamaName string) bool {
	if len(s.Params) == 0 {
		return false
	}

	last := s.Params[len(s.Params)-1]
	ellipsis, ok := last.Type.(*ast.Ellipsis)
	if !ok {
		return false
	}

	return isSelector(ellipsis.Elt, yamaName, optionTypeName)
}

// checkResults requires the three results the emitted constructor returns: the
// value the graph builds, the Lifecycle that orchestrates it, and the
// construction error.
func checkResults(fset *token.FileSet, s *Stub, yamaName string) error {
	want := fmt.Sprintf("results of the form (T, %s.%s, error)", yamaName, lifecycleTypeName)

	if len(s.Results) != stubResults {
		return newStubError(fset, s, s.FuncDecl.Type.Pos(), "declares %d results; it needs %s",
			len(s.Results), want)
	}
	if !isSelector(s.Results[1].Type, yamaName, lifecycleTypeName) {
		return newStubError(fset, s, s.Results[1].Type.Pos(), "second result is not %s.%s; it needs %s",
			yamaName, lifecycleTypeName, want)
	}
	if id, ok := s.Results[2].Type.(*ast.Ident); !ok || id.Name != "error" {
		return newStubError(fset, s, s.Results[2].Type.Pos(), "third result is not error; it needs %s", want)
	}

	return nil
}

// sortStubs orders stubs by file name, then by position, so a package's emitted
// constructors do not depend on the order the loader returned its files in.
func sortStubs(fset *token.FileSet, stubs []*Stub) {
	sort.SliceStable(stubs, func(i, j int) bool {
		pi := fset.Position(stubs[i].FuncDecl.Pos())
		pj := fset.Position(stubs[j].FuncDecl.Pos())
		if pi.Filename != pj.Filename {
			return pi.Filename < pj.Filename
		}

		return pi.Offset < pj.Offset
	})
}

// importName is the local name a file refers to an imported package by: the
// explicit alias when the import carries one, and the package's own declared
// name otherwise.
func importName(file *ast.File, path, pkgName string) (string, bool) {
	for _, spec := range file.Imports {
		unquoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil || unquoted != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name, true
		}

		return pkgName, true
	}

	return "", false
}

// isSelector reports whether expr is the qualified identifier pkg.name.
func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	id, ok := sel.X.(*ast.Ident)

	return ok && id.Name == pkg
}

// newStubError builds a *StubError with the position resolved against fset.
func newStubError(fset *token.FileSet, s *Stub, pos token.Pos, format string, args ...any) *StubError {
	return &StubError{
		Stub: s.Name,
		Pos:  fset.Position(pos),
		Msg:  fmt.Sprintf(format, args...),
	}
}
