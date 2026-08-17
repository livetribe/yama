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

package graph

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
)

// errName is the identifier that Google Wire binds an error to when the package
// block holds no name of its own for it.
const errName = "err"

// nilName is the identifier that an error check compares against.
const nilName = "nil"

// blank is the identifier that binds nothing.
const blank = "_"

// cleanupSuffix ends the name that renameCleanups gives a cleanup.
const cleanupSuffix = "Cleanup"

// An Injector is one function that Google Wire generated. Components hold the
// values it builds, in the order it builds them, which is a valid topological
// order.
//
// Statements is the construction that the injector performs, one entry for each
// line and printed as Google Wire wrote it. Result names the value the injector
// returns.
type Injector struct {
	Name       string
	Components []Component
	Statements []string
	Result     string
}

// Parse reads Google Wire's output and returns one Injector for each function
// that want names. Parse returns them in the order the file declares them, and
// it leaves out a name the file does not declare.
//
// Parse fills Name, Deps, and Cleanup. It leaves Capabilities empty, because a
// capability is a fact about a type, and Google Wire's output states only what
// the injector built.
func Parse(src []byte, want []string) ([]Injector, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "wire_gen.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse google wire's output: %w", err)
	}

	wanted := make(map[string]bool, len(want))
	for _, name := range want {
		wanted[name] = true
	}

	values := packageValues(file)

	var injectors []Injector

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !wanted[fn.Name.Name] {
			continue
		}

		components, err := componentsOf(fset, fn)
		if err != nil {
			return nil, err
		}

		if cleanupErr := checkCleanups(fset, fn, components); cleanupErr != nil {
			return nil, cleanupErr
		}

		bound, err := resolveValues(fset, fn, components, values)
		if err != nil {
			return nil, err
		}

		renameCleanups(fn, components)

		statements, result := bodyOf(fset, fn, bound)

		injectors = append(injectors, Injector{
			Name:       fn.Name.Name,
			Components: components,
			Statements: statements,
			Result:     result,
		})
	}

	return injectors, nil
}

// checkCleanups reports a cleanup that the injector's return aggregates and no
// component carries.
//
// Google Wire returns one function that calls every cleanup its providers
// returned. A lifecycle file states each of those calls at its own component's
// level, so a call that pairs with no component would reach no lifecycle, and
// the value it tears down would never be torn down.
func checkCleanups(fset *token.FileSet, fn *ast.FuncDecl, components []Component) error {
	closure := returnedClosure(fn)
	if closure == nil {
		return nil
	}

	carried := make(map[string]bool, len(components))

	for _, c := range components {
		if c.Cleanup != "" {
			carried[c.Cleanup] = true
		}
	}

	for name, pos := range calledNames(closure) {
		if !carried[name] {
			return rejected(fset, fn.Name.Name, pos, "the cleanup %s pairs with no component", name)
		}
	}

	return nil
}

// returnedClosure returns the function literal that the injector returns, which
// is the one that aggregates every cleanup. It returns nil when the injector
// returns none.
func returnedClosure(fn *ast.FuncDecl) *ast.FuncLit {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return nil
	}

	ret, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
	if !ok {
		return nil
	}

	for _, result := range ret.Results {
		if lit, ok := result.(*ast.FuncLit); ok {
			return lit
		}
	}

	return nil
}

// calledNames returns the name of each function that the closure calls by a
// bare name, with the position of the call.
func calledNames(closure *ast.FuncLit) map[string]token.Pos {
	called := make(map[string]token.Pos)

	ast.Inspect(closure.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := call.Fun.(*ast.Ident); ok {
			called[ident.Name] = ident.Pos()
		}

		return true
	})

	return called
}

// packageValues maps each package-level variable to what it was assigned.
// Google Wire writes a wire.Value provider and a wire.InterfaceValue provider
// as a read of one of these variables.
func packageValues(file *ast.File) map[string]ast.Expr {
	values := make(map[string]ast.Expr)

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range value.Names {
				if i < len(value.Values) {
					values[name.Name] = value.Values[i]
				}
			}
		}
	}

	return values
}

// resolveValues returns the line that states each read of a package-level
// variable that Google Wire wrote, keyed by the statement that reads it.
//
// The variable sits in Google Wire's output, and a run takes that file out of
// the package. A lifecycle file that read the variable would name something
// that no build declares, so the line states the value itself.
//
// resolveValues writes the line rather than the parsed statement. The value
// sits elsewhere in the file, and a printed statement carries the line breaks
// that the distance between the two implies.
//
// resolveValues reports a read that the file declares no value for. Such a read
// is a statement that this parser took for a value and cannot reproduce.
func resolveValues(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	components []Component,
	values map[string]ast.Expr,
) (map[ast.Stmt]string, error) {
	bound := make(map[ast.Stmt]string)

	if fn.Body == nil {
		return bound, nil
	}

	built := make(map[string]bool, len(components))
	for _, c := range components {
		built[c.Name] = true
	}

	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Rhs) != 1 {
			continue
		}

		read, ok := assign.Rhs[0].(*ast.Ident)
		if !ok || built[read.Name] {
			continue
		}

		value, found := values[read.Name]
		if !found {
			return nil, fmt.Errorf("%s reads %s, which google wire's output declares no value for", fn.Name.Name, read.Name)
		}

		name := printNode(fset, assign.Lhs[0])
		bound[stmt] = name + " := " + printNode(fset, value)
	}

	return bound, nil
}

// printNode prints one node as it was written.
func printNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer

	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}

	return buf.String()
}

// renameCleanups gives each cleanup the name of the component it tears down,
// with a "Cleanup" suffix. Google Wire names them cleanup, cleanup2, and so on,
// which says nothing about what each one tears down.
//
// renameCleanups changes the name in the parsed source and in the component, so
// the printed statements and the lifecycle agree.
func renameCleanups(fn *ast.FuncDecl, components []Component) {
	taken := make(map[string]bool, len(components))
	for _, c := range components {
		taken[c.Name] = true
	}

	renames := make(map[string]string)

	for i := range components {
		c := &components[i]
		if c.Cleanup == "" {
			continue
		}

		bound := freeName(c.Name+cleanupSuffix, taken)

		taken[bound] = true
		renames[c.Cleanup] = bound
		c.Cleanup = bound
	}

	if len(renames) == 0 {
		return
	}

	applyRenames(fn, renames)
}

// applyRenames gives every use of a renamed value its new name.
//
// applyRenames reads the left side of a selector and the value of a keyed
// field, so neither a field name nor a struct's own field key takes a rename.
func applyRenames(node ast.Node, renames map[string]string) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			applyRenames(e.X, renames)

			return false

		case *ast.KeyValueExpr:
			applyRenames(e.Value, renames)

			return false

		case *ast.Ident:
			if bound, found := renames[e.Name]; found {
				e.Name = bound
			}

			return false
		}

		return true
	})
}

// freeName returns want, or want with the lowest number that no name in taken
// already holds.
func freeName(want string, taken map[string]bool) string {
	if !taken[want] {
		return want
	}

	for i := 2; ; i++ {
		candidate := want + strconv.Itoa(i)

		if !taken[candidate] {
			return candidate
		}
	}
}

// bodyOf prints the construction that the injector performs, and it names the
// value the injector returns. It leaves the return statement out of the
// statements, because the lifecycle file returns something of its own.
func bodyOf(fset *token.FileSet, fn *ast.FuncDecl, bound map[ast.Stmt]string) (statements []string, result string) {
	if fn.Body == nil {
		return nil, ""
	}

	for _, stmt := range fn.Body.List {
		if ret, ok := stmt.(*ast.ReturnStmt); ok {
			return statements, firstResult(ret)
		}

		printed, ok := bound[stmt]
		if !ok {
			printed = printNode(fset, stmt)
		}

		for line := range strings.SplitSeq(printed, "\n") {
			statements = append(statements, line)
		}
	}

	return statements, ""
}

// firstResult names the value that a return statement returns first.
func firstResult(ret *ast.ReturnStmt) string {
	if len(ret.Results) == 0 {
		return ""
	}

	ident, ok := ret.Results[0].(*ast.Ident)
	if !ok {
		return ""
	}

	return ident.Name
}

// componentsOf reads one injector body. Google Wire binds each value it builds
// with a short variable declaration, and every other statement is an error
// check or the return.
//
// componentsOf reports a statement that it can derive no ordering from. Such a
// statement builds a value that would reach no lifecycle level, and the
// lifecycle would run without it.
func componentsOf(fset *token.FileSet, fn *ast.FuncDecl) ([]Component, error) {
	if fn.Body == nil {
		return nil, nil
	}

	body := fn.Body.List
	if len(body) == 0 {
		return nil, rejected(fset, fn.Name.Name, fn.Pos(), "the injector body is empty")
	}

	last := body[len(body)-1]
	if _, ok := last.(*ast.ReturnStmt); !ok {
		return nil, rejected(fset, fn.Name.Name, last.Pos(), "the injector body does not end in a return statement")
	}

	var components []Component

	known := make(map[string]bool)
	errs := errorNames(fn)

	for _, stmt := range body[:len(body)-1] {
		c, ok, err := componentOfStatement(fset, fn.Name.Name, stmt, known, errs)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		known[c.Name] = true
		components = append(components, c)
	}

	return components, nil
}

// componentOfStatement reads one statement that comes before the return. An
// error check states no value and contributes none.
//
// errs names every error that the injector binds.
func componentOfStatement(
	fset *token.FileSet,
	injector string,
	stmt ast.Stmt,
	known map[string]bool,
	errs map[string]bool,
) (Component, bool, error) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		return Component{}, false, nil

	case *ast.AssignStmt:
		if s.Tok != token.DEFINE {
			name := assignValueName(s)
			kind := assignKind(s)

			return Component{}, false, rejected(fset, injector, s.Pos(),
				"no traceable dependency edge: %s reaches the graph through a %s assignment", name, kind)
		}

		return componentOf(fset, injector, s, known, errs)

	default:
		kind := stmtKind(stmt)

		return Component{}, false, rejected(fset, injector, stmt.Pos(), "unsupported statement: %s", kind)
	}
}

// rejected names the injector and the position of the statement that Parse
// could derive no ordering from.
func rejected(fset *token.FileSet, injector string, pos token.Pos, format string, args ...any) error {
	where := fset.Position(pos)
	msg := fmt.Sprintf(format, args...)

	return fmt.Errorf("injector %s: %s: %s", injector, where, msg)
}

// stmtKind names the shape of an unsupported statement.
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

// exprKind names the shape of a right side that no provider writes.
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

// assignKind reports whether an assignment that declares nothing writes a field
// or a plain variable.
func assignKind(s *ast.AssignStmt) string {
	if _, ok := s.Lhs[0].(*ast.SelectorExpr); ok {
		return "field"
	}

	return "variable"
}

// assignValueName names the value that an assignment places. It falls back to a
// word of its own for a right side that is not a bare name.
func assignValueName(s *ast.AssignStmt) string {
	if len(s.Rhs) == 1 {
		if ident, ok := s.Rhs[0].(*ast.Ident); ok {
			return ident.Name
		}
	}

	return "a value"
}

// providerForm reports whether the right side of a declaration is a shape that
// one of Google Wire's provider kinds writes: a call, a read of a value, a
// struct literal that a build may take the address of, or a field of a value.
func providerForm(rhs ast.Expr) bool {
	switch e := rhs.(type) {
	case *ast.CallExpr, *ast.Ident, *ast.CompositeLit, *ast.SelectorExpr:
		return true

	case *ast.UnaryExpr:
		if e.Op != token.AND {
			return false
		}

		_, ok := e.X.(*ast.CompositeLit)

		return ok

	default:
		return false
	}
}

// componentOf reads one short variable declaration. known names every component
// that the injector already built, which is what tells a dependency apart from
// any other identifier. errs names every error that the injector binds, which
// is what tells an error apart from a cleanup.
func componentOf(
	fset *token.FileSet,
	injector string,
	assign *ast.AssignStmt,
	known map[string]bool,
	errs map[string]bool,
) (Component, bool, error) {
	names := identNames(assign.Lhs)
	if len(names) == 0 {
		return Component{}, false, rejected(fset, injector, assign.Pos(),
			"unrecognized provider form: the declaration binds something other than a name")
	}

	if len(assign.Rhs) != 1 {
		return Component{}, false, rejected(fset, injector, assign.Pos(),
			"unrecognized provider form: %s is bound to several values at once", names[0])
	}

	if names[0] == blank {
		return Component{}, false, nil
	}

	rhs := assign.Rhs[0]
	if !providerForm(rhs) {
		kind := exprKind(rhs)

		return Component{}, false, rejected(fset, injector, rhs.Pos(),
			"unrecognized provider form: %s is bound to a %s", names[0], kind)
	}

	c := Component{
		Name: names[0],
		Deps: consumed(assign.Rhs[0], known, nil),
	}

	for _, name := range names[1:] {
		if !errs[name] && name != blank {
			c.Cleanup = name

			break
		}
	}

	return c, true, nil
}

// errorNames returns every name that the injector body tests against nil. Google
// Wire writes that test after each provider that returns an error, and it binds
// every one of those errors to one name of its own.
//
// Google Wire takes another name when the target package block already holds
// "err". errorNames holds "err" as well, so output that states no test still
// reads the same way.
func errorNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{errName: true}

	if fn.Body == nil {
		return names
	}

	for _, stmt := range fn.Body.List {
		check, ok := stmt.(*ast.IfStmt)
		if !ok {
			continue
		}

		if name, found := nilTest(check.Cond); found {
			names[name] = true
		}
	}

	return names
}

// nilTest names the value that a condition of the form "x != nil" tests.
func nilTest(cond ast.Expr) (name string, ok bool) {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return "", false
	}

	tested, ok := binary.X.(*ast.Ident)
	if !ok {
		return "", false
	}

	against, ok := binary.Y.(*ast.Ident)
	if !ok || against.Name != nilName {
		return "", false
	}

	return tested.Name, true
}

// identNames returns the name of each identifier on the left of a declaration.
func identNames(lhs []ast.Expr) []string {
	names := make([]string, 0, len(lhs))

	for _, expr := range lhs {
		ident, ok := expr.(*ast.Ident)
		if !ok {
			return nil
		}

		names = append(names, ident.Name)
	}

	return names
}

// consumed returns every name in expr that names a component, in the order the
// expression holds them and without a repeat.
//
// consumed reads the left side of a selector and the value of a keyed field, so
// neither a field name nor a struct's own field key reads as a component.
func consumed(expr ast.Expr, known map[string]bool, into []string) []string {
	switch node := expr.(type) {
	case *ast.Ident:
		if known[node.Name] && !holds(into, node.Name) {
			into = append(into, node.Name)
		}
	case *ast.SelectorExpr:
		into = consumed(node.X, known, into)
	case *ast.KeyValueExpr:
		into = consumed(node.Value, known, into)
	case *ast.CallExpr:
		for _, arg := range node.Args {
			into = consumed(arg, known, into)
		}
	case *ast.CompositeLit:
		for _, elt := range node.Elts {
			into = consumed(elt, known, into)
		}
	case *ast.UnaryExpr:
		into = consumed(node.X, known, into)
	case *ast.StarExpr:
		into = consumed(node.X, known, into)
	case *ast.ParenExpr:
		into = consumed(node.X, known, into)
	case *ast.IndexExpr:
		into = consumed(node.X, known, into)
		into = consumed(node.Index, known, into)
	}

	return into
}

// holds reports whether names already holds want.
func holds(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}

	return false
}
