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

package pkg

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// An Import is one entry of an import block. Name is empty when the file gave
// no alias.
type Import struct {
	Name string
	Path string
}

// NameIn returns the name that a file uses for the import. The file's own alias
// has priority. The name that the package declares comes next. resolved holds
// that name for every path that the Go toolchain could read. If neither is
// available, NameIn makes a guess from the path itself.
func (imp Import) NameIn(resolved map[string]string) string {
	if imp.Name != "" {
		return imp.Name
	}

	if name, ok := resolved[imp.Path]; ok {
		return name
	}

	return PackageName("", imp.Path)
}

// PackageName returns the name that a file uses for an import. It is the alias
// when the import declares one. Otherwise it is the last element of the path,
// the element before a major-version element, or the last element without a
// major-version suffix.
func PackageName(name, path string) string {
	if name != "" {
		return name
	}

	elems := strings.Split(path, "/")

	last := elems[len(elems)-1]
	if len(elems) > 1 && majorVersion(last) {
		return elems[len(elems)-2]
	}

	return withoutVersionSuffix(last)
}

// ImportsIn returns the import block that src declares, in the order that src
// states it. It returns nothing for source that does not parse.
func ImportsIn(src []byte) []Import {
	file, err := parser.ParseFile(token.NewFileSet(), "imports.go", src, parser.ImportsOnly)
	if err != nil {
		return nil
	}

	block := make([]Import, 0, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		imp := Import{Path: path}
		if spec.Name != nil {
			imp.Name = spec.Name.Name
		}

		block = append(block, imp)
	}

	return block
}

// CheckImports reports one name that two paths use.
//
// A package states its imports one file at a time. Every file can bind a name
// of its own. Each file that Yama writes carries the imports of several such
// files at one time, and one name in the written file refers to one path. A
// caller passes the whole set that one written file will carry, so a run calls
// CheckImports one time for each file that it writes.
//
// CheckImports reads every import in that set, whether or not the written file
// goes on to state each one. A set can hold an import that the written file
// leaves out. That is the one case that CheckImports rejects and that Google
// Wire would have accepted. An alias on either import removes the collision.
func CheckImports(block []Import, names map[string]string) error {
	paths := make(map[string]string, len(block))

	for _, imp := range block {
		name := imp.NameIn(names)

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

// Qualifiers returns every name that src uses to qualify a selection. Those
// names are every package name that the code refers to. It returns an empty set
// for source that does not parse.
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

// withoutVersionSuffix removes a major-version suffix from a path element. A
// path under gopkg.in carries the version this way, as in "yaml.v3".
func withoutVersionSuffix(elem string) string {
	base, version, found := strings.Cut(elem, ".")
	if found && base != "" && majorVersion(version) {
		return base
	}

	return elem
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
