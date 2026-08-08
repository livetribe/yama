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

// newStubError builds a *StubError with the position resolved against fset.
func newStubError(fset *token.FileSet, s *Stub, pos token.Pos, format string, args ...any) *StubError {
	return &StubError{
		Stub: s.Name,
		Pos:  fset.Position(pos),
		Msg:  fmt.Sprintf(format, args...),
	}
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
