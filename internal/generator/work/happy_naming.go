// Copyright the original author or authors.
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

package work

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"

	"l7e.io/yama/internal/generator/graph"
	"l7e.io/yama/internal/generator/pkg"
)

// These are the paths of Yama's own two packages. Every lifecycle file imports
// them.
const (
	yamaPath = "l7e.io/yama"
	rtPath   = "l7e.io/yama/rt"
)

// blankName binds nothing. A parameter that carries it states no name that the
// constructor body can reach.
const blankName = "_"

// variadicPrefix starts the printed type of a variadic parameter.
const variadicPrefix = "..."

// naming holds the names that one lifecycle file uses for Yama's own two
// packages. It also holds the name that each constructor uses to forward its
// options.
//
// The lifecycle file shares a block with the package that contains it. Each
// constructor shares a scope with the values that its body builds. Yama's own
// imports cannot take a name that the block or a scope holds.
type naming struct {
	yama string
	rt   string

	// opts holds one name for each stub, in the same order as the package
	// declares the stubs. The entry is empty for a stub that declared no
	// options.
	opts []string
}

// fileImports holds the import block of one lifecycle file. It also holds each
// name that a stub file uses and that the lifecycle file changes.
type fileImports struct {
	block []pkg.Import

	// renamed maps a name that a stub file uses to the name that the
	// lifecycle file uses for the same package. It holds only the names that
	// change.
	renamed map[string]string
}

// nameFile decides what the lifecycle file calls Yama's own two packages, and
// what each constructor calls its options parameter.
//
// An options parameter takes a different name if any other name that its
// constructor reaches already holds it. Those other names are the names that
// scope holds, and every import. scope holds the names that fileScope returns.
// Yama's own two imports take a different name for all of those names, and for
// the options parameters also.
func nameFile(imports []pkg.Import, scope map[string]bool, stubs []pkg.Stub) naming {
	taken := maps.Clone(scope)
	if taken == nil {
		taken = make(map[string]bool)
	}

	// These are the names of the file's own import block. Yama's own two
	// imports are not in this set. nameFile decides the names that they
	// take.
	for _, imp := range imports {
		if ownedByYama(imp.Path) {
			continue
		}

		taken[imp.Name] = true
	}

	n := naming{opts: nameOptions(taken, stubs)}

	for _, name := range n.opts {
		if name != "" {
			taken[name] = true
		}
	}

	n.yama = freeName("yama", taken)
	taken[n.yama] = true

	n.rt = freeName("rt", taken)

	return n
}

// fileScope returns the names that the lifecycle file's scope holds before Yama
// names an import or an options parameter. Those names are the package block,
// every component, every cleanup, and every parameter that a stub declares
// beside its options.
func fileScope(injectors []graph.Injector, scope []string, stubs []pkg.Stub) map[string]bool {
	taken := make(map[string]bool, len(scope))
	for _, name := range scope {
		taken[name] = true
	}

	for _, inj := range injectors {
		for _, c := range inj.Components {
			taken[c.Name] = true

			if c.Cleanup != "" {
				taken[c.Cleanup] = true
			}
		}
	}

	takeParams(taken, stubs)

	return taken
}

// takeParams marks every parameter that a stub declares beside its options. A
// constructor keeps each of those names. Each of those names shares a scope
// with the constructor body.
func takeParams(taken map[string]bool, stubs []pkg.Stub) {
	for i := range stubs {
		for _, p := range stubs[i].GraphParams() {
			if p.Name == "" || p.Name == blankName {
				continue
			}

			taken[p.Name] = true
		}
	}
}

// nameOptions returns the name that each stub's constructor uses to forward
// its options. It returns an empty entry for a stub that declared no options.
//
// One constructor's options parameter shares a scope with no other
// constructor's options parameter. Each name must therefore differ only from
// the names in taken.
func nameOptions(taken map[string]bool, stubs []pkg.Stub) []string {
	opts := make([]string, len(stubs))

	for i := range stubs {
		name := stubs[i].OptionsName()
		if name == "" {
			continue
		}

		opts[i] = freeName(name, taken)
	}

	return opts
}

// nameImports decides the import block of the lifecycle file. stubs and output
// carry the name that refers to each entry.
//
// Every entry of Google Wire's output keeps its name. A stub file's entry keeps
// its name too, unless the output gives that name to a different package. Such
// an entry takes the name that the output gives the same package. When the
// output does not import that package, the entry takes its own name with the
// lowest free number. A new name takes none of the names that taken holds, and
// none of the names that an entry of either block holds. taken holds the names
// that fileScope returns.
//
// A stub file's entry for Google Wire's own package or for one of Yama's own
// two packages takes no part. A blank import and a dot import take no part
// either.
func nameImports(stubs, output []pkg.Import, taken map[string]bool) fileImports {
	result := fileImports{renamed: make(map[string]string)}

	// owner holds the path that each name in the import block refers to.
	owner := make(map[string]string, len(output)+len(stubs))

	// named holds the name that the block gives each path.
	named := make(map[string]string, len(output))

	// reserved holds every name that a new name must not take.
	reserved := make(map[string]bool, len(taken)+len(output)+len(stubs))
	maps.Copy(reserved, taken)

	for _, imp := range output {
		if !imp.BindsName() {
			continue
		}

		owner[imp.Name] = imp.Path
		named[imp.Path] = imp.Name
		reserved[imp.Name] = true
		result.block = append(result.block, imp)
	}

	entries := slices.DeleteFunc(slices.Clone(stubs), takesNoPart)
	for _, imp := range entries {
		reserved[imp.Name] = true
	}

	for _, imp := range entries {
		path, found := owner[imp.Name]

		switch {
		case !found:
			owner[imp.Name] = imp.Path
			result.block = append(result.block, imp)

			if named[imp.Path] == "" {
				named[imp.Path] = imp.Name
			}
		case path == imp.Path:
			// The block already holds this entry.
		case named[imp.Path] != "":
			result.renamed[imp.Name] = named[imp.Path]
		default:
			name := freeName(imp.Name, reserved)
			reserved[name] = true
			owner[name] = imp.Path
			named[imp.Path] = name
			result.block = append(result.block, pkg.Import{Name: name, Path: imp.Path})
			result.renamed[imp.Name] = name
		}
	}

	return result
}

// takesNoPart reports whether nameImports passes over a stub file's entry.
func takesNoPart(imp pkg.Import) bool {
	return !imp.BindsName() || ownedByYama(imp.Path) || imp.Path == pkg.WirePackagePath
}

// ownedByYama reports whether path is one of Yama's own two packages.
func ownedByYama(path string) bool {
	return path == yamaPath || path == rtPath
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

// requalify returns the type with each package name that names holds replaced
// by the name that names maps it to. It returns the type unchanged when the type
// names no such package.
//
// typ is a type that a stub states, as go/printer printed it. requalify panics
// when typ does not parse. Every such failure is a bug in Yama itself.
func requalify(typ string, names map[string]string) string {
	if len(names) == 0 {
		return typ
	}

	variadic := strings.HasPrefix(typ, variadicPrefix)
	src := strings.TrimPrefix(typ, variadicPrefix)

	fset := token.NewFileSet()

	expr, err := parser.ParseExprFrom(fset, "", src, 0)
	if err != nil {
		panic(fmt.Sprintf("work: the stub type %q does not parse: %v", typ, err))
	}

	if !renameQualifiers(expr, names) {
		return typ
	}

	var buf bytes.Buffer
	if variadic {
		buf.WriteString(variadicPrefix)
	}

	if err := printer.Fprint(&buf, fset, expr); err != nil {
		panic(fmt.Sprintf("work: the stub type %q does not print: %v", typ, err))
	}

	return buf.String()
}

// renameQualifiers replaces each package name in expr that names holds. It
// reports whether it replaced one. In a type, the left side of every selector is
// a package name.
func renameQualifiers(expr ast.Expr, names map[string]string) bool {
	changed := false

	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		qualifier, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if to, ok := names[qualifier.Name]; ok && to != qualifier.Name {
			qualifier.Name = to
			changed = true
		}

		return true
	})

	return changed
}
