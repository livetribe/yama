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
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// derivedFileName is the transient file Yama writes the derived injectors to.
// It is in the package directory, alongside the stub file. Wire's loader
// resolves a Go package by directory; a derived injector outside sp.Dir
// would not be part of the package Wire loads, and would not be generated.
const derivedFileName = "yama_wireinject.go"

// derivedFileMode is the permission for the transient derived-injector file.
const derivedFileMode = 0o600

// writeDerivedInjectors writes sp's derived injectors into the package
// directory. It returns the path of the file it wrote.
func writeDerivedInjectors(sp *StubPackage) (string, error) {
	content, err := deriveInjectors(sp)
	if err != nil {
		return "", err
	}

	target := filepath.Join(sp.Dir, derivedFileName)
	if err := os.WriteFile(target, content, derivedFileMode); err != nil {
		return "", fmt.Errorf("yama: writing %s in %s: %w", derivedFileName, sp.Dir, err)
	}

	return target, nil
}

// deriveInjectors renders one Google Wire injector per lifecycle stub.
//
// Each injector takes its name from its stub, in the reserved namespace. It
// takes its parameters from the stub's parameters, without the trailing Option
// parameter. Its results are the stub's first result, Wire's aggregated cleanup,
// and the construction error. Wire's output therefore always has a cleanup
// return for Yama to fold, whether or not a provider contributes one. Its body
// is the stub's body, copied unchanged.
func deriveInjectors(sp *StubPackage) ([]byte, error) {
	imports, err := derivedImports(sp)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n\n", generatedHeader)
	fmt.Fprintf(&buf, "//go:build %s\n\n", wireInjectTag)
	fmt.Fprintf(&buf, "package %s\n\n", sp.PkgName)
	writeImportBlock(&buf, imports)

	for _, stub := range sp.Stubs {
		params := printFields(sp.Fset, stub.GraphParams())
		result := printNode(sp.Fset, stub.ResultType())
		build := printNode(sp.Fset, stub.Build)

		buf.WriteString("\n")
		writeLineDirective(&buf, sp.Fset, stub)
		fmt.Fprintf(&buf, "func %s(%s) (%s, func(), error) {\n", stub.DerivedName(), params, result)
		fmt.Fprintf(&buf, "\tpanic(%s)\n}\n", build)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("yama: formatting derived injectors for %s: %w", sp.Dir, err)
	}

	return formatted, nil
}

// lineDirectivePrefix begins a Go line directive. It moves every position after
// it to the file the directive names.
const lineDirectivePrefix = "//line "

// writeLineDirective binds the derived injector that follows it to the stub the
// injector comes from.
//
// Google Wire loads the derived file through go/packages, which applies the
// directive. Wire then reports the stub's own file and position, in a file the
// application wrote, instead of the transient file's.
//
// Wire also copies the directive into its own output, as the generated
// injector's documentation. dropLineDirectives takes it back out before Yama
// reads a position from that output.
func writeLineDirective(buf *bytes.Buffer, fset *token.FileSet, stub *Stub) {
	pos := fset.Position(stub.FuncDecl.Pos())

	fmt.Fprintf(buf, "%s%s:%d:%d\n", lineDirectivePrefix, pos.Filename, pos.Line, pos.Column)
}

// dropLineDirectives blanks every line directive in src.
//
// Google Wire copies the directive Yama wrote into its generated output. A
// position Yama read from that output would resolve to the stub file, at a line
// the stub file does not hold: Wire's generated body holds one statement for
// each component, and the declaration the directive points into holds three
// lines. Yama's own diagnostics name the file they were read from instead.
//
// Each directive gives way to a comment of the same width, so every offset and
// every line number in src keeps the value it had.
func dropLineDirectives(src []byte) []byte {
	lines := bytes.Split(src, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if !bytes.HasPrefix(trimmed, []byte(lineDirectivePrefix)) {
			continue
		}

		blanked := bytes.Repeat([]byte(" "), len(line))
		copy(blanked, "//")
		lines[i] = blanked
	}

	return bytes.Join(lines, []byte("\n"))
}

// derivedImports maps each package name the derived injectors refer to onto its
// import path. It copies the signatures and the wire.Build calls, and nothing
// else. It therefore leaves out an import that a stub file carries for another
// purpose.
//
// Two stub files can give one name to two different packages. One derived file
// cannot use that name for both, so derivedImports reports the conflict rather
// than resolving it. The derived file is transient. A name Yama invented for the
// conflict would therefore appear in Wire's diagnostics, and in nothing an
// application can read.
func derivedImports(sp *StubPackage) (map[string]string, error) {
	imports := map[string]string{}
	for _, stub := range sp.Stubs {
		nodes := []ast.Node{stub.Build}
		for _, f := range stub.GraphParams() {
			nodes = append(nodes, f.Type)
		}
		nodes = append(nodes, stub.ResultType())

		declared := fileImports(stub.File, sp.ImportNames)
		for _, name := range qualifiers(nodes...) {
			importPath, ok := declared[name]
			if !ok {
				continue
			}
			if prev, seen := imports[name]; seen && prev != importPath {
				return nil, newStubError(sp.Fset, stub, stub.FuncDecl.Pos(),
					"package name %s refers to %s here and to %s in another stub file", name, importPath, prev)
			}
			imports[name] = importPath
		}
	}

	return imports, nil
}

// writeImportBlock writes an import declaration that names each package, ordered
// by path. Every import carries an explicit name. A path whose last element is
// not the package name therefore still resolves.
func writeImportBlock(buf *bytes.Buffer, imports map[string]string) {
	if len(imports) == 0 {
		return
	}

	names := make([]string, 0, len(imports))
	for name := range imports {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return imports[names[i]] < imports[names[j]]
	})

	buf.WriteString("import (\n")
	for _, name := range names {
		fmt.Fprintf(buf, "\t%s %s\n", name, strconv.Quote(imports[name]))
	}
	buf.WriteString(")\n")
}

// fileImports maps each package name that a file refers to onto the path that
// file imports.
//
// known gives an un-aliased import the name its own package declares, which the
// import path does not always carry: a path ending in a major-version suffix
// names no package at all. The loader maps every path a package imports, so
// known holds an entry for every import of a file in that package.
func fileImports(file *ast.File, known map[string]string) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		name := known[importPath]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}

	return imports
}

// qualifiers names the packages that a set of nodes refers to. The names are the
// base identifiers of every qualified identifier that those nodes contain. The
// result is sorted, so a caller that rejects a conflict reports the same conflict
// every run.
func qualifiers(nodes ...ast.Node) []string {
	seen := map[string]bool{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		ast.Inspect(n, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok {
				seen[id.Name] = true
			}

			return true
		})
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// printFields renders a parameter list. It keeps each name beside its own type,
// so a grouped declaration survives being read and written again.
func printFields(fset *token.FileSet, fields []Field) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		typ := printNode(fset, f.Type)
		if f.Name == "" {
			parts = append(parts, typ)
			continue
		}
		parts = append(parts, f.Name+" "+typ)
	}

	return strings.Join(parts, ", ")
}

// printNode renders one AST node as Go source. The only error that printing can
// report is the writer's error. A bytes.Buffer reports none. An unprintable node
// therefore yields empty text. The caller's format.Source then rejects that
// text.
func printNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}

	return buf.String()
}
