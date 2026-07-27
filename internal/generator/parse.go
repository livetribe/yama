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
	"go/token"
	"go/types"
)

// cleanupResultIndex is the position of the aggregated cleanup closure in an
// injector's return list, when the injector returns one.
const cleanupResultIndex = 1

// Parse walks the AST of a generated wire_gen.go and extracts one independent
// dependency graph per injector function. It performs no type analysis and
// derives no lifecycle levels; it records the ordered creation events, their
// dependency edges, each injector's returned root, and any Google Wire cleanup
// functions.
//
// A shape it cannot derive ordering from is reported as a *ParseError naming the
// injector and source position, never a panic and never a silent omission.
func Parse(fset *token.FileSet, file *ast.File) (*ParsedFile, error) {
	valueVars := packageValueVars(file)

	var injectors []*Injector
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		inj, err := parseInjector(fset, fn, valueVars)
		if err != nil {
			return nil, err
		}
		injectors = append(injectors, inj)
	}

	if err := checkResultNameCollisions(fset, injectors); err != nil {
		return nil, err
	}

	return &ParsedFile{Fset: fset, Syntax: file, Injectors: injectors}, nil
}

// parseInjector extracts one injector's graph. The body's final statement must be
// the return; every earlier statement is a creation event, an error guard that is
// skipped, or an unsupported shape that fails.
func parseInjector(fset *token.FileSet, fn *ast.FuncDecl, valueVars map[string]ast.Expr) (*Injector, error) {
	inj := &Injector{
		Name:     fn.Name.Name,
		Params:   extractFields(fn.Type.Params),
		Results:  extractFields(fn.Type.Results),
		FuncDecl: fn,
	}

	body := fn.Body.List
	if len(body) == 0 {
		return nil, newParseError(fset, inj.Name, fn.Pos(), "injector body is empty")
	}

	ret, ok := body[len(body)-1].(*ast.ReturnStmt)
	if !ok {
		return nil, newParseError(fset, inj.Name, body[len(body)-1].Pos(),
			"injector body does not end in a return statement")
	}
	inj.Return = ret

	byName := map[string]*Component{}
	for _, stmt := range body[:len(body)-1] {
		c, err := parseStatement(fset, inj.Name, stmt)
		if err != nil {
			return nil, err
		}
		if c != nil {
			inj.Components = append(inj.Components, c)
			byName[c.Name] = c
		}
	}

	deriveEdges(inj.Components, byName)
	if err := detectCleanups(fset, inj.Name, inj.Components, ret); err != nil {
		return nil, err
	}
	resolveValueExprs(inj.Components, valueVars)

	return inj, nil
}

// parseStatement classifies one non-final body statement. It returns a component
// for a creation event, nil for a skipped error guard, or an error for any shape
// the parser cannot derive ordering from.
func parseStatement(fset *token.FileSet, injector string, stmt ast.Stmt) (*Component, error) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE {
			return parseProviderForm(fset, injector, s)
		}
		return nil, newParseError(fset, injector, s.Pos(),
			"no traceable dependency edge: %s reaches the graph through a %s assignment",
			assignValueName(s), assignKind(s))
	case *ast.IfStmt:
		return nil, nil
	default:
		return nil, newParseError(fset, injector, stmt.Pos(),
			"unsupported statement: %s", stmtKind(stmt))
	}
}

// parseProviderForm turns a `:=` statement into a component. The first left-hand
// identifier is the value; the right-hand side must be a shape one of the four
// provider kinds Google Wire emits.
//
// Its errors report an unrecognized provider *form* rather than an unrecognized
// provider: no provider has been identified when they fire, and the shape itself
// is what went unrecognized.
func parseProviderForm(fset *token.FileSet, injector string, s *ast.AssignStmt) (*Component, error) {
	ident, ok := s.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, newParseError(fset, injector, s.Pos(),
			"unrecognized provider form: assignment target is not an identifier")
	}

	if len(s.Rhs) != 1 {
		return nil, newParseError(fset, injector, s.Pos(),
			"unrecognized provider form: %s is bound to multiple right-hand values", ident.Name)
	}

	rhs := s.Rhs[0]
	kind, ok := classifyProvider(rhs)
	if !ok {
		return nil, newParseError(fset, injector, rhs.Pos(),
			"unrecognized provider form: %s is bound to a %s", ident.Name, exprKind(rhs))
	}

	return &Component{Name: ident.Name, Ident: ident, Provider: kind, Rhs: rhs, Assign: s}, nil
}

// classifyProvider maps a creation right-hand side to the provider kind that
// emitted it. A struct literal may be address-taken (`&T{...}`); every other
// recognized kind is its bare node.
func classifyProvider(rhs ast.Expr) (ProviderKind, bool) {
	switch e := rhs.(type) {
	case *ast.CallExpr:
		return ProviderCall, true
	case *ast.Ident:
		return ProviderValue, true
	case *ast.CompositeLit:
		return ProviderStruct, true
	case *ast.SelectorExpr:
		return ProviderFieldsOf, true
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			if _, ok := e.X.(*ast.CompositeLit); ok {
				return ProviderStruct, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

// deriveEdges links each component to the components it consumes as inputs, kind
// by kind. Consumed identifiers that name a parameter or a package-level value
// are not components and contribute no edge.
func deriveEdges(components []*Component, byName map[string]*Component) {
	for _, c := range components {
		seen := map[*Component]bool{}
		for _, id := range consumedIdents(c) {
			dep, ok := byName[id.Name]
			if !ok || dep == c || seen[dep] {
				continue
			}
			seen[dep] = true
			c.Deps = append(c.Deps, dep)
		}
	}
}

// consumedIdents returns the identifiers a component's creation consumes as
// inputs, kind by kind. For a call, only the arguments are consumed, never the
// callee. collectIdents excludes field names, so a struct literal contributes its
// bound values and a field selection its base value.
func consumedIdents(c *Component) []*ast.Ident {
	switch c.Provider {
	case ProviderCall:
		return collectIdents(c.Rhs.(*ast.CallExpr).Args...)
	case ProviderFieldsOf:
		return collectIdents(c.Rhs)
	case ProviderStruct:
		return collectIdents(c.Rhs)
	case ProviderValue:
		return nil
	default:
		return nil
	}
}

// detectCleanups pairs each Google Wire cleanup function with the value it cleans
// up. The authoritative signal is membership in the aggregated cleanup closure the
// injector returns: every identifier that closure calls is a cleanup, and it
// belongs to the component bound in the same statement.
//
// A cleanup the closure calls but that pairs with no component is a shape the
// model cannot represent. It is reported rather than dropped, so a teardown is
// never silently lost.
func detectCleanups(fset *token.FileSet, injector string, components []*Component, ret *ast.ReturnStmt) error {
	if len(ret.Results) <= cleanupResultIndex {
		return nil
	}

	closure, ok := ret.Results[cleanupResultIndex].(*ast.FuncLit)
	if !ok {
		return nil
	}

	called := calledIdents(closure.Body)
	paired := map[string]bool{}
	for _, c := range components {
		for _, lhs := range c.Assign.Lhs[1:] {
			id, ok := lhs.(*ast.Ident)
			if ok && called[id.Name] {
				c.Cleanup = &Cleanup{Name: id.Name, Ident: id, Pos: id.Pos()}
				paired[id.Name] = true
			}
		}
	}

	for name := range called {
		if !paired[name] {
			return newParseError(fset, injector, closure.Pos(),
				"cleanup function %s pairs with no component", name)
		}
	}

	return nil
}

// resolveValueExprs attaches to each ProviderValue component the initializer of the
// package-level _wire*Value variable it reads, so the value can be re-emitted
// directly rather than referencing that private variable.
func resolveValueExprs(components []*Component, valueVars map[string]ast.Expr) {
	for _, c := range components {
		if c.Provider != ProviderValue {
			continue
		}
		if id, ok := c.Rhs.(*ast.Ident); ok {
			c.ValueExpr = valueVars[id.Name]
		}
	}
}

// checkResultNameCollisions rejects a file whose injectors return results that
// share an unqualified type name but denote different types: Yama's generated
// namespace derives from the unqualified name and cannot deterministically tell
// them apart. Injectors returning the identical result type are legal and share
// generated types.
func checkResultNameCollisions(fset *token.FileSet, injectors []*Injector) error {
	firstByName := map[string]*Injector{}
	for _, inj := range injectors {
		resultType := inj.ResultType()
		if resultType == nil {
			continue
		}

		short := unqualifiedTypeName(resultType)
		if short == "" {
			continue
		}

		prev, seen := firstByName[short]
		if !seen {
			firstByName[short] = inj
			continue
		}

		prevType := prev.ResultType()
		if types.ExprString(prevType) != types.ExprString(resultType) {
			return &ParseError{
				Injector: prev.Name + ", " + inj.Name,
				Pos:      fset.Position(resultType.Pos()),
				Msg:      "result type name collision on \"" + short + "\": distinct types share one generated namespace",
			}
		}
	}

	return nil
}
