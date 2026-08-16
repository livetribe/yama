// Package source reads the lifecycle stubs that a person writes by hand. A
// stub sits behind a build tag that no ordinary build sets, and its body is a
// single panic around a Google Wire provider list.
//
// Every value that source returns is a string, so a caller needs nothing from
// the Go toolchain to read one.
package source

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Tag is the build tag that guards every stub file. No ordinary build sets it.
const Tag = "yamainject"

// PackagePath is Yama's own package. A stub names its Option and its Lifecycle
// through this import.
const PackagePath = "l7e.io/yama/v2"

// WirePackagePath is Google Wire's own package. A stub states its providers in
// a call to this package's Build function.
const WirePackagePath = "github.com/google/wire"

// buildFunc is the function that a stub body calls.
const buildFunc = "Build"

// The types that a stub names through Yama's own package.
const (
	optionType    = "Option"
	lifecycleType = "Lifecycle"
)

// optionSuffix ends the type of the variadic parameter that carries options.
const optionSuffix = "." + optionType

// stubResults is the number of results that a stub declares.
const stubResults = 3

// errorType is the third result of every stub.
const errorType = "error"

// An Import is one entry of a stub file's import block. Name is empty when the
// file gave no alias.
type Import struct {
	Name string
	Path string
}

// A StubInfo is one lifecycle constructor that a person declared. Doc carries no
// comment markers. Providers holds one entry for each argument of the
// wire.Build call, printed as it was written.
type StubInfo struct {
	Name      string
	Doc       string
	Params    []Field
	Results   []Field
	Providers []string
	HasOpts   bool

	// Yama is the name that the stub's own file refers to Yama's public package
	// by. The stub's signature names its Option and its Lifecycle through it.
	Yama string

	// File names the file that declares the stub, and Line and Column are the
	// position of the declaration in that file. A file that Yama derives from
	// the stub carries this position, so the Go toolchain reports the stub
	// rather than the file Yama wrote.
	File   string
	Line   int
	Column int
}

// A Field is one parameter or one result. Name is empty when the declaration
// gave none.
type Field struct {
	Name string
	Type string
}

// GraphParams returns the parameters that the derived injector takes. These are
// the stub's own parameters, without a trailing variadic option parameter.
func (s *StubInfo) GraphParams() []Field {
	if !s.HasOpts {
		return s.Params
	}

	return s.Params[:len(s.Params)-1]
}

// ResultType returns the type of the value that the stub builds, which is the
// type of its first result. It is empty for a stub that declares no result.
func (s *StubInfo) ResultType() string {
	if len(s.Results) == 0 {
		return ""
	}

	return s.Results[0].Type
}

// Qualifiers returns every name that src uses to qualify a selection, which is
// every package name the code refers to. It returns an empty set for source
// that does not parse.
//
// Qualifiers reads the parsed source rather than the text, so neither a comment
// nor a field of a local value reads as a package.
func Qualifiers(src []byte) map[string]bool {
	used := make(map[string]bool)

	file, err := parser.ParseFile(token.NewFileSet(), "body.go", src, 0)
	if err != nil {
		return used
	}

	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if ident, ok := sel.X.(*ast.Ident); ok {
			used[ident.Name] = true
		}

		return true
	})

	return used
}

// PackageName is the name that a file refers to an import by. It is the alias
// when the import declares one. Otherwise it is the last element of the path,
// and the element before a major-version element.
func PackageName(name, path string) string {
	if name != "" {
		return name
	}

	elems := strings.Split(path, "/")

	last := elems[len(elems)-1]
	if len(elems) > 1 && majorVersion(last) {
		return elems[len(elems)-2]
	}

	return last
}

// majorVersion reports whether an import path element names a major version,
// which is a "v" and then one or more digits.
func majorVersion(elem string) bool {
	digits, ok := strings.CutPrefix(elem, "v")
	if !ok || digits == "" {
		return false
	}

	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// ImportName is the name that a file refers to an import by. The file's own
// alias wins. The name that the package declares comes next, which names holds
// for every path the Go toolchain could read. A guess from the path itself is
// the last resort.
func ImportName(imp Import, names map[string]string) string {
	if imp.Name != "" {
		return imp.Name
	}

	if name, ok := names[imp.Path]; ok {
		return name
	}

	return PackageName("", imp.Path)
}

// checkImports reports one name that two paths answer to.
//
// A package states its imports one file at a time, and every file may bind a
// name of its own. Yama puts the imports of every stub file in one derived
// file, and one name there answers to one path.
//
// checkImports reads every import of the package, and Go rejects a file that
// imports a package it never uses. A guarded file that uses an import outside
// its stubs is the one case that checkImports rejects and Google Wire would
// have taken. An alias on either import settles it.
func checkImports(imports []Import, names map[string]string) error {
	paths := make(map[string]string, len(imports))

	for _, imp := range imports {
		name := ImportName(imp, names)

		seen, found := paths[name]
		if !found {
			paths[name] = imp.Path

			continue
		}

		if seen != imp.Path {
			return fmt.Errorf("the name %s refers to %s and to %s; one of them needs an alias", name, seen, imp.Path)
		}
	}

	return nil
}

// Load reads every stub that dir declares. Load returns an empty PackageInfo
// when the directory declares no stub, and it reports no error for that.
//
// Load reads the stub files alone, so it fills neither PkgPath nor ImportNames.
// Both state what the Go toolchain says about a package, and CollectPackageInfo
// asks the toolchain for them.
//
// tags are the build tags that the run set. A stub file may name one of them
// beside Tag, and Load reads such a file only when the run set that tag.
func Load(dir string, tags []string) (*PackageInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	pkg := &PackageInfo{Dir: dir, ImportNames: make(map[string]string)}
	fset := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}

		file, err := parseTagged(fset, filepath.Join(dir, name), tags)
		if err != nil {
			return nil, err
		}

		if file == nil {
			continue
		}

		stubs, err := stubsOf(fset, file)
		if err != nil {
			return nil, err
		}

		pkg.Name = file.Name.Name
		pkg.Stubs = append(pkg.Stubs, stubs...)
		pkg.Imports = append(pkg.Imports, importsOf(file)...)
	}

	return pkg, nil
}

// parseTagged returns the parsed file when its build constraint holds under
// Tag. It returns nil for every other file.
//
// parseTagged reads the build header before it parses. A package holds files
// that are none of source's business, and one of those must never fail a load.
func parseTagged(fset *token.FileSet, path string, tags []string) (*ast.File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}

	if !guardedByTag(src, tags) {
		return nil, nil
	}

	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}

	return file, nil
}

// guardedByTag reports whether Tag is what puts the file in a build. The
// constraint must hold with Tag set and fail with Tag clear. The run's own tags
// are set either way, so a file that names one of them beside Tag still counts.
//
// A constraint that holds with Tag clear guards nothing of source's, whatever
// else it names. `!wireinject` is one of those, and it is the constraint that
// Google Wire puts on the file it writes.
func guardedByTag(src []byte, tags []string) bool {
	set := make(map[string]bool, len(tags))
	for _, tag := range tags {
		set[tag] = true
	}

	for _, line := range headerLines(src) {
		expr, err := constraint.Parse(line)
		if err != nil {
			continue
		}

		withTag := expr.Eval(func(tag string) bool { return tag == Tag || set[tag] })
		withoutTag := expr.Eval(func(tag string) bool { return set[tag] })

		if withTag && !withoutTag {
			return true
		}
	}

	return false
}

// headerLines returns the line comments of a file's header. A header holds
// blank lines, line comments, and block comments. It ends at the first line
// that is none of those, which is the package clause of a Go file.
//
// A block comment contributes no line. A build constraint is a line comment.
func headerLines(src []byte) []string {
	var lines []string

	inBlock := false

	for line := range strings.SplitSeq(string(src), "\n") {
		trimmed := strings.TrimSpace(line)

		if inBlock {
			rest, closed := afterBlock(trimmed)
			if !closed {
				continue
			}

			trimmed = rest
		}

		trimmed, inBlock = skipBlocks(trimmed)

		if inBlock || trimmed == "" {
			continue
		}

		if !strings.HasPrefix(trimmed, "//") {
			return lines
		}

		lines = append(lines, trimmed)
	}

	return lines
}

// skipBlocks takes every block comment that starts and ends on this line off
// the front of it. open reports that the last block runs on to the next line.
func skipBlocks(line string) (rest string, open bool) {
	for strings.HasPrefix(line, "/*") {
		body := strings.TrimPrefix(line, "/*")

		after, closed := afterBlock(body)
		if !closed {
			return "", true
		}

		line = after
	}

	return line, false
}

// afterBlock returns what follows the end of a block comment on one line.
func afterBlock(s string) (rest string, closed bool) {
	_, after, found := strings.Cut(s, "*/")
	if !found {
		return "", false
	}

	return strings.TrimSpace(after), true
}

// stubsOf returns every stub that the file declares, in the order it declares
// them. It reports a stub whose signature Yama cannot emit a constructor from.
//
// A declaration is a stub when its body is one panic around a wire.Build call.
// Every other declaration is none of source's business, and stubsOf checks the
// signature of a stub alone.
func stubsOf(fset *token.FileSet, file *ast.File) ([]StubInfo, error) {
	wire, imported := importedAs(file, WirePackagePath)
	if !imported {
		return nil, nil
	}

	yama, _ := importedAs(file, PackagePath)

	var stubs []StubInfo

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}

		build, ok := buildCall(fn, wire)
		if !ok {
			continue
		}

		stub := stubOf(fset, fn, build, yama)

		if err := checkStub(&stub); err != nil {
			return nil, err
		}

		stubs = append(stubs, stub)
	}

	return stubs, nil
}

// importedAs is the name that the file refers to the path by, and whether the
// file imports it at all. A file that imports the path without an alias refers
// to it by the name that its package declares.
func importedAs(file *ast.File, path string) (string, bool) {
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil || imported != path {
			continue
		}

		if spec.Name != nil {
			return spec.Name.Name, true
		}

		return PackageName("", path), true
	}

	return PackageName("", path), false
}

// checkStub reports whether the stub declares the signature that an emitted
// constructor needs: the value the graph builds, the Lifecycle that runs it,
// and the construction error. An option parameter comes last and is variadic.
func checkStub(stub *StubInfo) error {
	lifecycle := stub.Yama + "." + lifecycleType
	want := fmt.Sprintf("results of the form (T, %s, %s)", lifecycle, errorType)

	switch {
	case len(stub.Results) != stubResults:
		return stubError(stub, "declares %d results; it needs %s", len(stub.Results), want)

	case stub.Results[1].Type != lifecycle:
		return stubError(stub, "declares %s as its second result; it needs %s", stub.Results[1].Type, want)

	case stub.Results[2].Type != errorType:
		return stubError(stub, "declares %s as its third result; it needs %s", stub.Results[2].Type, want)
	}

	return checkOptions(stub)
}

// checkOptions reports a final option parameter that is not variadic. A final
// parameter that is variadic and is not an option is an ordinary graph
// parameter, and so is a parameter of any other type.
func checkOptions(stub *StubInfo) error {
	if len(stub.Params) == 0 {
		return nil
	}

	option := stub.Yama + optionSuffix

	last := stub.Params[len(stub.Params)-1].Type
	if last != option {
		return nil
	}

	return stubError(stub, "declares %s as its final parameter; it needs a variadic ...%s", last, option)
}

// stubError names the stub and the position that declares it.
func stubError(stub *StubInfo, format string, args ...any) error {
	where := fmt.Sprintf("%s:%d:%d: stub %s ", stub.File, stub.Line, stub.Column, stub.Name)

	return fmt.Errorf(where+format, args...)
}

// stubOf reads one stub out of its declaration.
func stubOf(fset *token.FileSet, fn *ast.FuncDecl, build *ast.CallExpr, yama string) StubInfo {
	params := fieldsOf(fset, fn.Type.Params)
	pos := fset.Position(fn.Pos())

	stub := StubInfo{
		Name:      fn.Name.Name,
		Doc:       docOf(fn.Doc),
		Params:    params,
		Results:   fieldsOf(fset, fn.Type.Results),
		Providers: argsOf(fset, build),
		HasOpts:   endsWithOptions(params),
		Yama:      yama,
		File:      pos.Filename,
		Line:      pos.Line,
		Column:    pos.Column,
	}

	return stub
}

// buildCall returns the wire.Build call of a stub body. A stub declares one
// statement, and that statement is a panic around the call.
func buildCall(fn *ast.FuncDecl, wire string) (*ast.CallExpr, bool) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return nil, false
	}

	stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}

	panicCall, ok := callTo(stmt.X, "panic")
	if !ok || len(panicCall.Args) != 1 {
		return nil, false
	}

	build, ok := callTo(panicCall.Args[0], buildFunc)
	if !ok {
		return nil, false
	}

	if !qualifiedBy(build.Fun, wire) {
		return nil, false
	}

	return build, true
}

// qualifiedBy reports whether the called function is the named package's own.
// A call to a Build function of any other package states no provider list, and
// the declaration that holds it is no stub.
func qualifiedBy(fun ast.Expr, pkg string) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == pkg
}

// callTo returns the call when expr calls a function of the given name. The
// name is the identifier itself, or the selected name of a qualified call.
func callTo(expr ast.Expr, name string) (*ast.CallExpr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return call, fun.Name == name
	case *ast.SelectorExpr:
		return call, fun.Sel.Name == name
	}

	return nil, false
}

// fieldsOf flattens a parameter or result list. One declaration that names
// several identifiers becomes one entry for each name.
func fieldsOf(fset *token.FileSet, list *ast.FieldList) []Field {
	if list == nil {
		return nil
	}

	var fields []Field

	for _, field := range list.List {
		typ := printNode(fset, field.Type)

		if len(field.Names) == 0 {
			fields = append(fields, Field{Type: typ})

			continue
		}

		for _, ident := range field.Names {
			fields = append(fields, Field{Name: ident.Name, Type: typ})
		}
	}

	return fields
}

// importsOf reads the file's import block, in the order the file declares it.
func importsOf(file *ast.File) []Import {
	imports := make([]Import, 0, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		imp := Import{Path: path}
		if spec.Name != nil {
			imp.Name = spec.Name.Name
		}

		imports = append(imports, imp)
	}

	return imports
}

// argsOf prints every argument of the call, as it was written.
func argsOf(fset *token.FileSet, call *ast.CallExpr) []string {
	args := make([]string, 0, len(call.Args))

	for _, arg := range call.Args {
		args = append(args, printNode(fset, arg))
	}

	return args
}

// printNode prints one node as it was written.
//
// printNode prints the parsed node rather than a short form of it. A short form
// leaves the elements of a composite literal out, and an argument that names a
// provider carries every element that the person wrote.
func printNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer

	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}

	return buf.String()
}

// docOf joins a doc comment into plain lines, without the comment markers.
func docOf(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}

	return strings.TrimRight(group.Text(), "\n")
}

// endsWithOptions reports whether the parameter list ends in a variadic option
// parameter.
func endsWithOptions(params []Field) bool {
	if len(params) == 0 {
		return false
	}

	last := params[len(params)-1].Type

	return strings.HasPrefix(last, "...") && strings.HasSuffix(last, optionSuffix)
}
