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

// packageValueVars maps each package-level variable to its initializer. Google
// Wire emits wire.Value and wire.InterfaceValue providers as reads of these
// _wire*Value variables.
func packageValueVars(file *ast.File) map[string]ast.Expr {
	vars := map[string]ast.Expr{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}

		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					vars[name.Name] = vs.Values[i]
				}
			}
		}
	}

	return vars
}

// extractFields flattens a parameter or result list, expanding each grouped
// declaration into one Field per name and keeping unnamed results.
func extractFields(list *ast.FieldList) []Field {
	if list == nil {
		return nil
	}

	var fields []Field
	for _, f := range list.List {
		if len(f.Names) == 0 {
			fields = append(fields, Field{Type: f.Type})
			continue
		}
		for _, name := range f.Names {
			fields = append(fields, Field{Name: name.Name, Type: f.Type})
		}
	}

	return fields
}

// collectIdents gathers the identifiers an expression references as values,
// skipping identifiers that name fields rather than values: the field name of a
// selector (only the base of a chain is a value), the key of a composite-literal
// element, and a composite literal's type name. This keeps a dependency edge from
// being minted for a struct field or a projected field that merely shares a
// component's name.
func collectIdents(exprs ...ast.Expr) []*ast.Ident {
	var ids []*ast.Ident
	for _, e := range exprs {
		collectValueIdents(e, &ids)
	}

	return ids
}

func collectValueIdents(e ast.Expr, ids *[]*ast.Ident) {
	if e == nil {
		return
	}

	ast.Inspect(e, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			*ids = append(*ids, node)
		case *ast.SelectorExpr:
			collectValueIdents(node.X, ids)
			return false
		case *ast.KeyValueExpr:
			collectValueIdents(node.Value, ids)
			return false
		case *ast.CompositeLit:
			for _, elt := range node.Elts {
				collectValueIdents(elt, ids)
			}
			return false
		}
		return true
	})
}

// calledIdents returns the set of identifiers invoked as bare calls (id()) in a
// block. It is how a cleanup closure names the cleanup functions it aggregates.
func calledIdents(block *ast.BlockStmt) map[string]bool {
	called := map[string]bool{}
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok {
			called[id.Name] = true
		}
		return true
	})

	return called
}

// unqualifiedTypeName is the bare type name of a (possibly pointer or
// package-qualified) type expression, or "" if it has no simple name.
func unqualifiedTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return unqualifiedTypeName(e.X)
	case *ast.ParenExpr:
		return unqualifiedTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

// newParseError builds a *ParseError with the position resolved against fset.
func newParseError(fset *token.FileSet, injector string, pos token.Pos, format string, args ...any) *ParseError {
	return &ParseError{
		Injector: injector,
		Pos:      fset.Position(pos),
		Msg:      fmt.Sprintf(format, args...),
	}
}

// exprKind names the shape of an unrecognized creation right-hand side for an
// error message, so the diagnostic is specific to what the parser saw.
func exprKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.BinaryExpr:
		return "binary expression"
	case *ast.IndexExpr, *ast.IndexListExpr:
		return "index expression"
	case *ast.TypeAssertExpr:
		return "type assertion"
	case *ast.StarExpr:
		return "pointer dereference"
	case *ast.UnaryExpr:
		return "unary expression"
	case *ast.SliceExpr:
		return "slice expression"
	case *ast.FuncLit:
		return "function literal"
	case *ast.BasicLit:
		return "literal value"
	case *ast.ParenExpr:
		return "parenthesized expression"
	default:
		return "unsupported expression"
	}
}

// stmtKind names the shape of an unsupported statement for an error message.
func stmtKind(stmt ast.Stmt) string {
	switch stmt.(type) {
	case *ast.ExprStmt:
		return "expression statement"
	case *ast.DeclStmt:
		return "local declaration"
	case *ast.ForStmt, *ast.RangeStmt:
		return "loop"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return "switch statement"
	case *ast.GoStmt:
		return "go statement"
	case *ast.DeferStmt:
		return "defer statement"
	default:
		return "statement"
	}
}

// assignKind reports whether a non-creation assignment writes a field or a plain
// variable, distinguishing an untraceable field write from a reassignment.
func assignKind(s *ast.AssignStmt) string {
	if _, ok := s.Lhs[0].(*ast.SelectorExpr); ok {
		return "field"
	}

	return "variable"
}

// assignValueName names the value an untraceable assignment places, for an error
// message; it falls back to a generic word when the right-hand side is not a bare
// identifier.
func assignValueName(s *ast.AssignStmt) string {
	if len(s.Rhs) == 1 {
		if id, ok := s.Rhs[0].(*ast.Ident); ok {
			return id.Name
		}
	}

	return "a value"
}
