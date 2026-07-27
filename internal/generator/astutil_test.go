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
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The malformed fixtures trip only a couple of arms of the diagnostic mappers
// below, so the rest are exercised here directly. Every arm is reachable from a
// hand-authored wire_gen.go, and these mappers are what make the parser's errors
// shape-specific, so a dropped or duplicated arm is a real regression a table
// pins down.

// TestExprKind covers the phrase for each unrecognized provider-form right-hand
// side. The arms are the expression shapes classifyProvider rejects; only a
// binary RHS is reached by a fixture.
func TestExprKind(t *testing.T) {
	cases := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{"binary", &ast.BinaryExpr{}, "binary expression"},
		{"index", &ast.IndexExpr{}, "index expression"},
		{"index list", &ast.IndexListExpr{}, "index expression"},
		{"type assertion", &ast.TypeAssertExpr{}, "type assertion"},
		{"star", &ast.StarExpr{}, "pointer dereference"},
		{"unary", &ast.UnaryExpr{}, "unary expression"},
		{"slice", &ast.SliceExpr{}, "slice expression"},
		{"func literal", &ast.FuncLit{}, "function literal"},
		{"basic literal", &ast.BasicLit{}, "literal value"},
		{"parenthesized", &ast.ParenExpr{}, "parenthesized expression"},
		{"unlisted shape", &ast.KeyValueExpr{}, "unsupported expression"},
	}

	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, exprKind(tc.expr))
		})

		// Distinct shapes must read distinctly, except the two index forms and
		// the catch-all, which deliberately share a phrase.
		if tc.want != "index expression" && tc.want != "unsupported expression" {
			if prev, dup := seen[tc.want]; dup {
				t.Errorf("%q and %q both map to %q", prev, tc.name, tc.want)
			}
			seen[tc.want] = tc.name
		}
	}
}

// TestStmtKind covers the phrase for each unsupported body statement. The arm
// reached by a fixture is the field-write assignment, which parseStatement
// handles before stmtKind, so every arm here is otherwise unexercised.
func TestStmtKind(t *testing.T) {
	cases := []struct {
		name string
		stmt ast.Stmt
		want string
	}{
		{"expression", &ast.ExprStmt{}, "expression statement"},
		{"declaration", &ast.DeclStmt{}, "local declaration"},
		{"for loop", &ast.ForStmt{}, "loop"},
		{"range loop", &ast.RangeStmt{}, "loop"},
		{"switch", &ast.SwitchStmt{}, "switch statement"},
		{"type switch", &ast.TypeSwitchStmt{}, "switch statement"},
		{"go", &ast.GoStmt{}, "go statement"},
		{"defer", &ast.DeferStmt{}, "defer statement"},
		{"unlisted shape", &ast.SendStmt{}, "statement"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, stmtKind(tc.stmt))
		})
	}
}

// TestAssignKind covers both arms: a field write, which a fixture reaches, and a
// plain variable reassignment, which none does.
func TestAssignKind(t *testing.T) {
	field := &ast.AssignStmt{Lhs: []ast.Expr{&ast.SelectorExpr{Sel: ast.NewIdent("base2")}}}
	variable := &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("x")}}

	assert.Equal(t, "field", assignKind(field))
	assert.Equal(t, "variable", assignKind(variable))
}

// TestAssignValueName covers the bare-identifier right-hand side and the generic
// fallback for everything else.
func TestAssignValueName(t *testing.T) {
	ident := &ast.AssignStmt{Rhs: []ast.Expr{ast.NewIdent("base2")}}
	nonIdent := &ast.AssignStmt{Rhs: []ast.Expr{&ast.CallExpr{}}}
	multiple := &ast.AssignStmt{Rhs: []ast.Expr{ast.NewIdent("a"), ast.NewIdent("b")}}

	assert.Equal(t, "base2", assignValueName(ident))
	assert.Equal(t, "a value", assignValueName(nonIdent), "a non-identifier right-hand side has no name")
	assert.Equal(t, "a value", assignValueName(multiple), "a multi-value assignment has no single name")
}

// TestUnqualifiedTypeName covers the recursion through pointer and parenthesized
// types down to a bare or package-qualified name, and the empty result for a type
// with no simple name.
func TestUnqualifiedTypeName(t *testing.T) {
	qualified := &ast.SelectorExpr{X: ast.NewIdent("inbound"), Sel: ast.NewIdent("App")}

	cases := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{"bare identifier", ast.NewIdent("App"), "App"},
		{"package qualified", qualified, "App"},
		{"pointer", &ast.StarExpr{X: ast.NewIdent("App")}, "App"},
		{"parenthesized pointer", &ast.ParenExpr{X: &ast.StarExpr{X: ast.NewIdent("App")}}, "App"},
		{"no simple name", &ast.ArrayType{Elt: ast.NewIdent("byte")}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, unqualifiedTypeName(tc.expr))
		})
	}
}
