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
	"fmt"
	"go/ast"
	"go/token"
)

// ProviderKind is the kind of Google Wire provider that produced a
// component's value: a provider function, a value, a struct provider, or a
// field provider, in Wire's own terms. These four are the only kinds that
// Wire emits a creation event for. Any other statement shape in the
// injector body is a parse error.
//
// The shape of the statement that Wire emitted tells the kinds apart.
// ProviderValue covers both wire.Value and wire.InterfaceValue. They are two
// distinct Wire providers, but both emit an assignment from a package-level
// _wire*Value variable, and both are identical in the injector body. What
// separates them lives in that variable's initializer, which
// Component.ValueExpr carries for both.
type ProviderKind int

const (
	// ProviderCall is a call to a provider function,
	// e.g. consoleLogger := NewConsoleLogger(w, cfg).
	ProviderCall ProviderKind = iota
	// ProviderValue is a wire.Value or wire.InterfaceValue, emitted as an
	// assignment from a package-level _wire*Value variable, e.g. config := _wireConfigValue.
	ProviderValue
	// ProviderStruct is a wire.Struct provider, emitted as a struct literal that can be
	// address-taken, e.g. cfg := &Cfg{A: a, B: b}.
	ProviderStruct
	// ProviderFieldsOf is a wire.FieldsOf provider, emitted as a field selection,
	// e.g. environment := config.Env.
	ProviderFieldsOf
)

func (k ProviderKind) String() string {
	switch k {
	case ProviderCall:
		return "call"
	case ProviderValue:
		return "value"
	case ProviderStruct:
		return "struct"
	case ProviderFieldsOf:
		return "fieldsOf"
	default:
		return fmt.Sprintf("ProviderKind(%d)", int(k))
	}
}

// ParsedFile is the result of parsing one generated wire_gen.go. It owns the
// FileSet and *ast.File that the model's AST nodes belong to. go/types keys
// its result maps on node identity, so resolving a Component's type requires
// the very Syntax/Fset parsed here, not a reparse.
type ParsedFile struct {
	Fset      *token.FileSet
	Syntax    *ast.File
	Injectors []*Injector
}

// Field is one entry in an injector's parameter or result list. Name is empty for
// an unnamed result.
type Field struct {
	Name string
	Type ast.Expr
}

// Injector is one injector function's dependency graph, extracted from the
// generated wire_gen.go. Each injector is an independent graph. Yama never
// merges injectors.
//
// Components are in statement order. Google Wire guarantees that this order
// is a valid topological order: a component's dependencies precede it.
//
// Results is the signature's declared result list. The value that the graph
// builds is its first entry, which ResultType names and Return yields.
//
// FuncDecl and Return are the re-emission source. The body is reproduced
// verbatim up to, but not including, Return. Return's own value is then
// swapped for the generated one.
type Injector struct {
	Name       string
	Params     []Field
	Results    []Field
	Components []*Component

	FuncDecl *ast.FuncDecl
	Return   *ast.ReturnStmt
}

// ResultName is the injector-local variable that the returned value is bound
// to, or "" when no variable names it: an injector that returns nothing, or
// one that returns an expression rather than an identifier. An empty name is
// unambiguous, since a bound value always has one.
//
// A caller that must reproduce the return reads Return. A caller that only
// reports the result substitutes its own placeholder.
func (i *Injector) ResultName() string {
	if i.Return == nil || len(i.Return.Results) == 0 {
		return ""
	}

	id, ok := i.Return.Results[0].(*ast.Ident)
	if !ok {
		return ""
	}

	return id.Name
}

// ResultType is the type of the value that the graph builds. ResultType
// takes this type from the signature's first declared result, or returns nil
// when the signature declares none. This type is authoritative even when the
// value came from a provider call.
func (i *Injector) ResultType() ast.Expr {
	if len(i.Results) == 0 {
		return nil
	}

	return i.Results[0].Type
}

// Component is one value created by a statement in the injector body.
//
// Name is the injector-local variable that the value is bound to (the first
// left-hand-side identifier of its assignment). Ident is that identifier,
// kept to resolve the value's concrete type by node identity. Deps points at
// the other components that this one consumes: a call's arguments, a struct
// literal's field values, or a field selection's base value. The pointers
// stay valid when a consumer filters Components to a subset, which raw
// indices would not.
//
// Yama retains a Component even when its value implements no lifecycle
// interface. A dependency-only value still orders the components that depend
// on it, and a value that only carries a Cleanup still tears down. This
// package does not decide which components participate in a lifecycle
// operation.
type Component struct {
	Name     string
	Ident    *ast.Ident
	Provider ProviderKind
	Rhs      ast.Expr
	Assign   *ast.AssignStmt
	Deps     []*Component

	// ValueExpr is the resolved initializer of a ProviderValue component's
	// _wire*Value variable (for example os.Stdout or Config{...}), read from the
	// file's package-level var block. It is nil for every other kind.
	ValueExpr ast.Expr

	// Cleanup is the Google Wire cleanup function that this value's provider
	// returned, or nil. Yama folds Cleanup into this value's teardown. Cleanup
	// is not a component of its own.
	Cleanup *Cleanup
}

// Cleanup is the Google Wire cleanup function that a provider returned
// alongside its value. Pos locates the binding.
type Cleanup struct {
	Name  string
	Ident *ast.Ident
	Pos   token.Pos
}

// ParseError reports an injector shape that the parser cannot derive
// lifecycle ordering from. It names the injector and the offending source
// position. Because of this, the failure is a build-time error with a
// location, rather than a panic or a silent mis-ordering.
type ParseError struct {
	Injector string
	Pos      token.Position
	Msg      string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("yama: injector %s: %s: %s", e.Injector, e.Pos, e.Msg)
}
