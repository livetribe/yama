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
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireDiagnosticNamesTheStub asserts a Google Wire input error reaches the
// application naming the file, the position, and the function the application
// itself wrote.
//
// Wire runs against the derived injector, and Yama removes that file before
// anything is printed. A diagnostic carrying its position names a path the
// application cannot open, at a line that belongs to no file it has, for a
// function it never declared.
func TestWireDiagnosticNamesTheStub(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "wirefail")
	stub := filepath.Join(dir, "lifecycle.go")

	_, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"."})

	var toolErr *ToolError
	require.ErrorAs(t, err, &toolErr)

	line := declLine(t, stub, "NewLifecycle")
	want := fmt.Sprintf("lifecycle.go:%d:1: inject NewLifecycle:", line)

	msg := err.Error()
	assert.Contains(t, msg, want, "the diagnostic locates the stub's own declaration")
	assert.NotContains(t, msg, derivedFileName, "the transient file is never named")
	assert.NotContains(t, msg, derivedPrefix, "no derived identifier reaches the application")
}

// TestParseIgnoresTheLineDirectiveWireCopied asserts a position Yama reads out
// of Google Wire's output names the file Yama read.
//
// Wire copies Yama's line directive into its output as the generated injector's
// documentation. Left in place, it sends every position after it into the stub
// file, at a line derived from a three-line declaration. Wire's generated body
// holds one statement for each component, so those lines run past the end of
// the stub file and name code that has nothing to do with the diagnostic.
func TestParseIgnoresTheLineDirectiveWireCopied(t *testing.T) {
	src := []byte("package p\n\n//line /abs/lifecycle.go:26:1\nfunc yama_New() {\n\tdep := NewDep()\n}\n")

	g := NewGenerator(Options{})

	fset := token.NewFileSet()
	file, err := g.parseWithoutLineDirectives(fset, "wire_gen.go", src)
	require.NoError(t, err)

	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)

	pos := fset.Position(fn.Body.List[0].Pos())
	assert.Equal(t, "wire_gen.go", pos.Filename, "the position names the file it was read from")
	assert.Equal(t, 5, pos.Line, "the position keeps the line it holds in that file")
}

// TestParseKeepsTheLineDirectiveInEveryOtherFile asserts a line directive
// survives parsing in any file besides the one Google Wire writes.
//
// A package under load may hold a file with its own reason to carry a `//line
// ` prefixed line. Blanking it there would corrupt content the derived-injector
// mechanism has no business touching.
func TestParseKeepsTheLineDirectiveInEveryOtherFile(t *testing.T) {
	src := []byte("package p\n\n//line /abs/other.go:26:1\nfunc F() {}\n")

	g := NewGenerator(Options{})

	fset := token.NewFileSet()
	file, err := g.parseWithoutLineDirectives(fset, "components.go", src)
	require.NoError(t, err)

	pos := fset.Position(file.Decls[0].Pos())
	assert.Equal(t, filepath.Clean("/abs/other.go"), pos.Filename, "the directive still applies outside wire_gen.go")
}

// TestDropLineDirectivesKeepsEveryOffset asserts blanking a directive changes
// neither the length of the source nor the number of lines in it, so every
// position that follows keeps the value it had.
func TestDropLineDirectivesKeepsEveryOffset(t *testing.T) {
	src := []byte("package p\n\n//line /abs/lifecycle.go:26:1\nvar x = 1\n")

	got := dropLineDirectives(src)

	assert.Len(t, got, len(src), "the source keeps its length")
	assert.Equal(t, bytes.Count(src, []byte("\n")), bytes.Count(got, []byte("\n")))
	assert.NotContains(t, string(got), lineDirectivePrefix, "no directive survives")
	assert.Contains(t, string(got), "var x = 1", "every other line is untouched")
}

// TestDropLineDirectivesLeavesIndentedCommentsAlone asserts a line carrying the
// prefix anywhere but column 1 survives.
//
// Go honors a directive at column 1 alone, so an indented line that begins with
// the same characters is ordinary comment text. Blanking it would destroy
// content while correcting nothing.
func TestDropLineDirectivesLeavesIndentedCommentsAlone(t *testing.T) {
	src := []byte("package p\n\nfunc F() {\n\t//line up the columns before printing\n}\n")

	got := dropLineDirectives(src)

	assert.Equal(t, string(src), string(got), "an indented comment is not a directive")
}

// TestDropLineDirectivesLeavesStringContentAlone asserts a line inside a string
// literal survives, however it begins.
//
// A wire.Value provider carries its value expression into Wire's output, and
// the emitted constructor reproduces that expression. Blanking a line inside
// one writes a corrupted value into the committed lifecycle file, which
// compiles and reports nothing.
func TestDropLineDirectivesLeavesStringContentAlone(t *testing.T) {
	src := []byte("package p\n\nvar banner = `usage:\n//line up the columns\ndone`\n")

	got := dropLineDirectives(src)

	assert.Equal(t, string(src), string(got), "string content is not a comment")
}

// declLine is the 1-based line that path declares name on.
func declLine(t *testing.T, path, name string) int {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	for i, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "func "+name+"(") {
			return i + 1
		}
	}

	t.Fatalf("no declaration of %s in %s", name, path)

	return 0
}

// TestDeriveInjectorsMirrorsTheStub asserts the derived injector Google Wire is
// given: the stub's name in the reserved namespace, the stub's parameters
// without its options, Wire's own cleanup and error results, and the stub's
// wire.Build call unchanged.
func TestDeriveInjectorsMirrorsTheStub(t *testing.T) {
	requireGo(t)

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), "testapp")
	require.NoError(t, err)

	derived, err := deriveInjectors(sp)
	require.NoError(t, err)

	src := string(derived)
	assert.Contains(t, src, "//go:build wireinject")
	assert.Contains(t, src, "func yama_NewLifecycle(rec *Recorder, fault Fault) (*Top, func(), error)")
	assert.Contains(t, src, "panic(wire.Build(GraphSet))")

	injector := declarationOf(t, src, "yama_NewLifecycle")
	assert.NotContains(t, injector, "yama.Lifecycle", "the Lifecycle result is replaced by Wire's cleanup")
	assert.NotContains(t, injector, optionTypeName, "the options parameter reaches no injector")
}

// TestDerivedFileIsTransient asserts a generation run leaves neither transient
// file behind, and creates no backup of the derived-injector file.
func TestDerivedFileIsTransient(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "chain")

	_, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"."})
	require.NoError(t, err)

	for _, name := range []string{derivedFileName, wireGenBaseName, backupNameFor(derivedFileName)} {
		_, statErr := os.Stat(filepath.Join(dir, name))
		assert.True(t, os.IsNotExist(statErr), "%s survived the run", name)
	}
}

// TestEmitOverwritesAStrandedDerivedFile asserts a derived-injector file left by
// an interrupted run does not stop the next one. Yama owns that filename.
func TestEmitOverwritesAStrandedDerivedFile(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "chain")
	path := filepath.Join(dir, derivedFileName)
	stranded := []byte("//go:build wireinject\n\npackage chain\n\nfunc Stranded() {}\n")

	require.NoError(t, os.WriteFile(path, stranded, 0o600))
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	files, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"."})
	require.NoError(t, err)
	require.Len(t, files, 1)

	assert.NoFileExists(t, path, "the stranded file is removed with the run's own")
}

// TestEmitRegeneratesOverAStaleLifecycleFile asserts a committed lifecycle file
// that no longer compiles does not stop the run that replaces it.
//
// A provider rename leaves the committed file referring to a symbol that no
// longer exists.
func TestEmitRegeneratesOverAStaleLifecycleFile(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "chain")
	path := filepath.Join(dir, lifecycleGenName)
	stale := []byte("//go:build !yamainject\n\npackage chain\n\nfunc Stale() *C { return NewRenamedC() }\n")

	require.NoError(t, os.WriteFile(path, stale, 0o600))
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	files, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"."})
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Contains(t, string(files[0].Content), "func NewLifecycle(")

	restored, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(stale), string(restored),
		"the run returns the file rather than writing it, so the committed copy is put back untouched")
}

// TestDeriveKeepsAnImportWhosePathDoesNotNameIt asserts the derived injector
// file carries the imports its signatures refer to.
//
// A package imported without an alias, under a path whose last element is a
// version suffix, is referred to by the name the package declares. Taking that
// name from the path instead leaves the import out of the derived file, and
// Wire then rejects it for an undefined identifier in a file the application
// never wrote.
func TestDeriveKeepsAnImportWhosePathDoesNotNameIt(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "versioned")

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)
	require.NoError(t, err)

	derived, err := deriveInjectors(sp)
	require.NoError(t, err)

	src := string(derived)
	assert.Contains(t, src, `thing "l7e.io/yama/v2/internal/generator/testdata/emit/versioned/v2"`)
	assert.Contains(t, src, "func yama_NewLifecycle(c *thing.Client)")
}

// TestDeriveWritesAPlaceholderPerStub asserts the derived file declares the
// constructor its stub names, under the whole declared signature, beside the
// injector.
//
// A caller passes the arguments the stub declares and reads the results it
// declares. A declaration that drops a parameter or a result does not compile
// against that caller.
func TestDeriveWritesAPlaceholderPerStub(t *testing.T) {
	requireGo(t)

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), "testapp")
	require.NoError(t, err)

	derived, err := deriveInjectors(sp)
	require.NoError(t, err)

	src := string(derived)
	assert.Contains(t, src, "func NewLifecycle(rec *Recorder, fault Fault, opts ...yama.Option) (*Top, yama.Lifecycle, error) {")
	assert.Contains(t, src, "func yama_NewLifecycle(rec *Recorder, fault Fault) (*Top, func(), error) {")
	assert.Contains(t, src, `yama "l7e.io/yama/v2"`)
}

// TestPlaceholderCarriesALineDirective asserts each placeholder is bound to the
// stub it comes from.
//
// The derived file is transient. A diagnostic carrying its own position names a
// path the application cannot open.
func TestPlaceholderCarriesALineDirective(t *testing.T) {
	requireGo(t)

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), "testapp")
	require.NoError(t, err)

	derived, err := deriveInjectors(sp)
	require.NoError(t, err)

	line := declLine(t, filepath.Join("testapp", "lifecycle.go"), "NewLifecycle")
	want := fmt.Sprintf("lifecycle.go:%d:1", line)

	src := string(derived)
	assert.Equal(t, 2, strings.Count(src, lineDirectivePrefix), "one directive per declaration")
	assert.Contains(t, src, want)
}

// TestEmitResolvesASiblingPackagesConstructor asserts a run over two stub
// packages generates both, when an ordinary file in one calls the constructor
// the other's stub declares.
//
// Google Wire type-checks every package it loads. The caller's package fails to
// compile unless the constructor resolves while the committed lifecycle file is
// held aside.
func TestEmitResolvesASiblingPackagesConstructor(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "crosspkg")

	files, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"./..."})
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// declarationOf is the source text of one function declaration in src, from its
// func keyword to the closing brace in the first column.
func declarationOf(t *testing.T, src, name string) string {
	t.Helper()

	open := strings.Index(src, "func "+name+"(")
	require.GreaterOrEqual(t, open, 0, "no declaration of %s", name)

	rest := src[open:]
	end := strings.Index(rest, "\n}\n")
	require.GreaterOrEqual(t, end, 0, "declaration of %s does not close", name)

	return rest[:end]
}
