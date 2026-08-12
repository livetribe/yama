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
	"errors"
	"fmt"
	"go/ast"
	"go/token"
)

// yamaInjectTag gates a lifecycle stub file. Yama loads the file under this
// tag to see the stubs. Yama marks its own generated file with `!yamainject`.
// Because of this, the stub and the emitted constructor never compile
// together.
const yamaInjectTag = "yamainject"

// derivedPrefix begins the name of every injector that Yama derives from a
// stub. An identifier that carries this prefix is reserved. An application
// must not declare a Wire injector with a name that starts with this prefix.
const derivedPrefix = "yama_"

// wirePkgPath is the import path of Google Wire's public package. A stub
// states its providers in a call to this package's Build function.
// wirePkgName is the name that this package declares.
const (
	wirePkgPath = "github.com/google/wire"
	wirePkgName = "wire"
)

// yamaPkgPath is the import path of Yama's public package. A stub's
// signature names this package's Option and Lifecycle types. yamaPkgName is
// the name that this package declares.
//
// The last element of the path is the major-version suffix, not the package
// name. Because of this, a file that imports the path without an alias still
// refers to the package as yamaPkgName.
const (
	yamaPkgPath = "l7e.io/yama/v2"
	yamaPkgName = "yama"
)

// optionTypeName is the variadic parameter type that can end a stub's
// parameter list.
const optionTypeName = "Option"

// defaultOptsName is the identifier that the emitted constructor uses to
// forward its options when the stub binds no usable name to the parameter.
const defaultOptsName = "opts"

// blankIdent is Go's blank identifier. Generated code cannot read a name
// from a parameter that is bound to the blank identifier.
const blankIdent = "_"

// lifecycleTypeName is the second result that a stub declares.
const lifecycleTypeName = "Lifecycle"

// stubResults is the number of results that a stub declares: the value that
// the graph builds, the Lifecycle that orchestrates it, and the construction
// error.
const stubResults = 3

// ErrNoStubs reports that a package declares no lifecycle stub. This
// condition is not a failure. A sweep skips such a package. The sweep does
// not report the package.
var ErrNoStubs = errors.New("yama: package has no lifecycle stub")

// StubError reports a lifecycle stub that Yama cannot derive an injector
// from. StubError names the stub and the source position. Because of this,
// the failure is a build-time error with a location.
type StubError struct {
	Stub string
	Pos  token.Position
	Msg  string
}

func (e *StubError) Error() string {
	return fmt.Sprintf("yama: lifecycle stub %s: %s: %s", e.Stub, e.Pos, e.Msg)
}

// newStubError builds a *StubError with the position resolved against fset.
func newStubError(fset *token.FileSet, s *Stub, pos token.Pos, format string, args ...any) *StubError {
	return &StubError{
		Stub: s.Name,
		Pos:  fset.Position(pos),
		Msg:  fmt.Sprintf(format, args...),
	}
}

// Stub is one hand-authored lifecycle stub. It holds the constructor name
// and signature that an application declares. It also holds the wire.Build
// call that states the providers for its graph.
//
// Params holds the whole declared parameter list. This list includes the
// options parameter. Results holds all three declared results. File is the
// stub's own file. This file's imports name the packages that the signature
// refers to.
//
// HasOpts reports whether the parameter list ends in a variadic yama.Option.
// A stub that ends this way gets a constructor that takes options. A stub
// that does not end this way gets a constructor that takes no options.
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

// DerivedName is the injector that Yama derives from this stub.
func (s *Stub) DerivedName() string {
	return derivedPrefix + s.Name
}

// GraphParams are the parameters that the derived injector takes. These are
// the stub's own parameters, without a trailing variadic Option parameter.
func (s *Stub) GraphParams() []Field {
	if !s.HasOpts {
		return s.Params
	}

	return s.Params[:len(s.Params)-1]
}

// OptsName is the identifier that the emitted constructor uses to forward
// its options. This is the name that the stub bound to the parameter. If the
// stub bound no name, or bound the blank identifier, OptsName is
// defaultOptsName. Call OptsName only when HasOpts is set.
func (s *Stub) OptsName() string {
	last := s.Params[len(s.Params)-1]
	if last.Name == "" || last.Name == blankIdent {
		return defaultOptsName
	}

	return last.Name
}

// ResultType is the type of the value that the graph builds. Yama takes this
// type from the stub's first declared result.
func (s *Stub) ResultType() ast.Expr {
	return s.Results[0].Type
}

// StubPackage holds the lifecycle stubs that one package declares, in a
// stable order: by file name, then by position within the file.
type StubPackage struct {
	Dir     string
	PkgName string
	PkgPath string
	Fset    *token.FileSet
	Stubs   []*Stub

	// ImportNames maps each package that the stub file imports to that
	// package's declared name. The loader resolves this name. The loader does
	// not guess the name from the import path.
	//
	// An un-aliased import can declare a name that differs from the last
	// element of its path. A module path with a version suffix is one example.
	// Without this map, the derived injector file misnames such an import.
	ImportNames map[string]string
}
