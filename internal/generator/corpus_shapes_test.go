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
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/graph"
	"l7e.io/yama/v2/internal/generator/resolve"
	"l7e.io/yama/v2/internal/generator/wire"
	"l7e.io/yama/v2/internal/generator/work"
)

// wireShapes names every statement shape that Google Wire writes at the top
// level of an injector body.
//
// graph.Parse reads an injector body at that level alone. It takes a short
// variable declaration whose right side is a shape that one of Google Wire's
// provider kinds writes. It steps over a condition. It rejects every other
// shape and names it.
//
// A shape that Google Wire writes and this list does not hold reaches
// graph.Parse as a rejection. Add the shape here after graph.Parse takes it.
//
// graph.Parse takes one more shape than this list holds: a right side that is a
// composite literal with no address operator. A provider of a struct value
// writes that shape. No fixture states such a provider.
var wireShapes = []string{
	"define 1 <- &composite-lit",
	"define 1 <- call",
	"define 1 <- ident",
	"define 1 <- selector",
	"define 2 <- call",
	"define 3 <- call",
	"if",
	"return",
}

// shapeFixtures names each fixture that states an injector, and the arguments
// that its own build tags need. Together they state every provider kind that
// Google Wire accepts.
var shapeFixtures = []struct {
	name string
	args wire.Args
}{
	{name: "sandbox"},
	{name: "capabilities"},
	{name: "cleanup"},
	{name: "minimal"},
	{name: "noyama"},
	{name: "structcomp"},
	{name: "crosspkg"},
	{name: "multipkg"},
	{name: "tagged", args: wire.Args{Tags: "special"}},
	{name: "emit/value"},
	{name: "emit/empty"},
	{name: "emit/fanout"},
	{name: "emit/chain"},
	{name: "emit/cleanup"},
	{name: "emit/diamond"},
	{name: "emit/mixed"},
	{name: "emit/pkgscope"},
	{name: "emit/versioned"},
	{name: "emit/variadicgraphparam"},
}

// TestWireEmitsOnlyTheShapesTheParserReads runs Google Wire over every fixture
// that states an injector. It reads the shape of each statement that Google
// Wire wrote at the top level of an injector body.
//
// The shapes that Google Wire wrote must be the shapes that wireShapes names. A
// new shape fails this test, and the failure names the file that holds it. A
// shape that no fixture holds fails this test too.
func TestWireEmitsOnlyTheShapesTheParserReads(t *testing.T) {
	requireGo(t)

	found := make(map[string]string)

	for _, fixture := range shapeFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			dir := copyFamily(t, fixture.name)

			gatherShapes(t, dir, fixture.args, found)
		})
	}

	got := slices.Sorted(maps.Keys(found))

	assert.Equalf(t, wireShapes, got,
		"Google Wire wrote a set of statement shapes that graph.Parse does not state\nfirst file per shape: %v", found)
}

// gatherShapes runs Google Wire over the copy in dir. It puts each statement
// shape that Google Wire wrote into found, against the file that first held it.
func gatherShapes(t *testing.T, dir string, args wire.Args, found map[string]string) {
	t.Helper()

	injectors := injectorNames(t, dir)

	runWireOverCopy(t, dir, args)

	outputs := wireOutputFiles(t, dir)
	require.NotEmpty(t, outputs, "Google Wire wrote no output over this fixture")

	for _, path := range outputs {
		present := readShapes(t, path, injectors, found)

		requireParserReads(t, path, present)
	}
}

// requireParserReads gives one output file to graph.Parse. Every shape that
// this test collected from that file must be a shape that graph.Parse reads.
func requireParserReads(t *testing.T, path string, present []string) {
	t.Helper()

	if len(present) == 0 {
		return
	}

	src, err := os.ReadFile(path)
	require.NoError(t, err)

	body := wire.DropLineDirectives(src)

	_, err = graph.Parse(body, present)
	require.NoErrorf(t, err, "graph.Parse rejected a shape that Google Wire wrote in %s", path)
}

// runWireOverCopy invokes Google Wire one time over the copy in dir.
//
// A fixture that declares a lifecycle stub takes the injectors that a run
// derives, which is the path that an application takes. Every other fixture
// takes the injectors that it states itself.
//
// Google Wire rejects the graph of some fixtures. This test reads the output of
// every other package of the same run, so it takes that rejection.
func runWireOverCopy(t *testing.T, dir string, args wire.Args) {
	t.Helper()
	t.Chdir(dir)

	ctx := context.Background()
	patterns := []string{"./..."}
	tags := args.TagList()

	targets, err := resolve.Packages(ctx, ".", stubTags(tags), patterns)
	require.NoError(t, err)

	items := work.CreateWorkItems(targets, args.Prefix, nil, tags, io.Discard)
	for i, item := range items {
		items[i] = item.Prepare()
	}

	t.Cleanup(func() {
		for _, item := range items {
			_ = item.Complete()
		}
	})

	derived := items.Paths()
	if len(derived) == 0 {
		derived = patterns
	}

	_, _ = wire.Run(ctx, ".", derived, args)
}

// readShapes puts the shape of each statement of one output file into found. It
// returns the name of each injector that the file holds.
//
// injectors names the functions that state a Google Wire build. Google Wire
// copies every other declaration of the injector file, and graph.Parse reads
// none of them.
func readShapes(t *testing.T, path string, injectors map[string]bool, found map[string]string) []string {
	t.Helper()

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	require.NoError(t, err)

	var present []string

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !injectors[fn.Name.Name] {
			continue
		}

		present = append(present, fn.Name.Name)

		for _, stmt := range fn.Body.List {
			shape := shapeOf(stmt)
			if _, held := found[shape]; !held {
				found[shape] = filepath.Base(path) + ":" + fn.Name.Name
			}
		}
	}

	return present
}

// injectorNames reads every source file of the copy in dir. It returns the name
// of each function that states a Google Wire build.
func injectorNames(t *testing.T, dir string) map[string]bool {
	t.Helper()

	names := make(map[string]bool)
	fset := token.NewFileSet()

	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && buildsAGraph(fn) {
				names[fn.Name.Name] = true
			}
		}

		return nil
	}))

	require.NotEmpty(t, names, "the fixture states no injector")

	return names
}

// wireOutputFiles returns the path of every file that Google Wire wrote under
// dir.
func wireOutputFiles(t *testing.T, dir string) []string {
	t.Helper()

	var paths []string

	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		if entry.Name() == wire.BaseOutputName {
			paths = append(paths, path)
		}

		return nil
	}))

	return paths
}

// buildsAGraph reports whether a function states a Google Wire build.
func buildsAGraph(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	states := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Build" {
			return true
		}

		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "wire" {
			states = true
		}

		return true
	})

	return states
}

// shapeOf names the shape of one statement of an injector body. It names each
// shape that graph.Parse reads. It names every other shape by its syntax type.
func shapeOf(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return assignShape(s)
	case *ast.IfStmt:
		return "if"
	case *ast.ReturnStmt:
		return "return"
	default:
		return fmt.Sprintf("%T", stmt)
	}
}

// assignShape names the shape of one assignment. It states the number of names
// that the left side binds, and the shape of the right side.
func assignShape(s *ast.AssignStmt) string {
	verb := "define"
	if s.Tok != token.DEFINE {
		verb = "assign"
	}

	if len(s.Rhs) != 1 {
		return verb + " <- several values"
	}

	arity := strconv.Itoa(len(s.Lhs))
	right := rightSideShape(s.Rhs[0])

	return verb + " " + arity + " <- " + right
}

// rightSideShape names the shape of the right side of one assignment. It names
// each shape that a Google Wire provider kind writes. It names every other
// shape by its syntax type.
func rightSideShape(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		return "call"
	case *ast.Ident:
		return "ident"
	case *ast.CompositeLit:
		return "composite-lit"
	case *ast.SelectorExpr:
		return "selector"
	case *ast.UnaryExpr:
		_, holdsLiteral := e.X.(*ast.CompositeLit)
		if holdsLiteral && e.Op == token.AND {
			return "&composite-lit"
		}

		return fmt.Sprintf("%T", expr)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
