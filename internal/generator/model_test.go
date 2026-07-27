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

// TestInjectorResultName asserts the shapes that have no result variable all
// report "", which is what tells a caller to reach for Return instead.
//
// The injectors are built by hand rather than parsed: every fixture in the tree
// returns a named variable, so none of the empty cases arises from real Wire
// output.
func TestInjectorResultName(t *testing.T) {
	cases := []struct {
		name string
		ret  *ast.ReturnStmt
		want string
	}{
		{
			name: "result bound to a variable",
			ret:  &ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("app")}},
			want: "app",
		},
		{
			// A bare `return`, which yields no expression to name.
			name: "injector has no results",
			ret:  &ast.ReturnStmt{},
			want: "",
		},
		{
			// `return &App{...}`: an expression is returned, so no variable names it.
			name: "result is an inline return expression",
			ret:  &ast.ReturnStmt{Results: []ast.Expr{&ast.CompositeLit{}}},
			want: "",
		},
		{
			// The zero value, which no parse produces: it must not panic.
			name: "injector has no return statement",
			ret:  nil,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inj := &Injector{Name: "InitializeApp", Return: tc.ret}

			assert.Equal(t, tc.want, inj.ResultName())
		})
	}
}

// TestInjectorResultType asserts the type comes from the signature's first
// declared result, and that a signature declaring none yields nil rather than a
// zero-valued expression.
func TestInjectorResultType(t *testing.T) {
	app := ast.NewIdent("App")

	withResults := &Injector{Results: []Field{{Type: app}, {Type: ast.NewIdent("error")}}}
	assert.Equal(t, app, withResults.ResultType())

	assert.Nil(t, (&Injector{}).ResultType())
}
