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
	"fmt"
	"go/ast"
	"go/token"
	"sort"

	"golang.org/x/tools/go/packages"
)

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

	hasOpts, err := hasOptsDeclared(fset, s, yamaName)
	if err != nil {
		return nil, err
	}
	s.HasOpts = hasOpts

	if err := checkResults(fset, s, yamaName); err != nil {
		return nil, err
	}

	return s, nil
}

// hasOptsDeclared reports whether the stub's parameter list ends in a variadic
// Option parameter. The derived injector takes the stub's parameters without
// that one, and the emitted constructor forwards it to the builder.
//
// A final parameter that is variadic but not Option is an ordinary graph
// parameter. A final parameter that is Option but not variadic is a
// *StubError.
func hasOptsDeclared(fset *token.FileSet, s *Stub, yamaName string) (bool, error) {
	if len(s.Params) == 0 {
		return false, nil
	}

	last := s.Params[len(s.Params)-1]

	if ellipsis, ok := last.Type.(*ast.Ellipsis); ok {
		return isSelector(ellipsis.Elt, yamaName, optionTypeName), nil
	}

	if isSelector(last.Type, yamaName, optionTypeName) {
		want := fmt.Sprintf("a final parameter of the form opts ...%s.%s", yamaName, optionTypeName)
		return false, newStubError(fset, s, last.Type.Pos(), "final parameter is %s.%s but not variadic; it needs %s",
			yamaName, optionTypeName, want)
	}

	return false, nil
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
