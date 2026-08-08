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
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireGo(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not found on PATH: %v", err)
	}
}

// generateFixture runs Google Wire over the package in dir and returns the named
// injectors, parsed and type-checked. The directory is left as it was found,
// whether or not the run succeeds.
//
// Generation proper reads lifecycle stubs, and never an application's own
// wire.go. The parse and analysis fixtures state their graphs as plain
// injectors, which keeps each one to the shape under test, so the tests that
// read them drive the same machinery from here instead.
func generateFixture(ctx context.Context, g *Generator, dir string, names []string) (pkg *LoadedPackage, err error) {
	scopes, err := openWireGenScopes([]string{dir}, g.wireGenName)
	if err != nil {
		return nil, err
	}

	defer func() {
		if restoreErr := scopes.restore(); restoreErr != nil {
			pkg, err = nil, errors.Join(err, restoreErr)
		}
	}()

	if wireErr := g.runWire(ctx, dir, []string{"."}, nil); wireErr != nil {
		return nil, wireErr
	}

	return g.LoadInjectors(ctx, dir, names)
}

// TestLoadResolvesEveryComponentType loads and type-checks a package holding
// Wire's output, and asserts that every component identifier the parser recorded
// resolves against the package's type information. This is the node-identity
// contract the model depends on: it type-checks these exact nodes rather than
// reparsing.
func TestLoadResolvesEveryComponentType(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "structcomp")

	pkg, err := NewGenerator(Options{}).LoadInjectors(ctx, dir, []string{"InitRoot"})
	require.NoError(t, err)
	require.Len(t, pkg.Injectors, 1)
	require.NotNil(t, pkg.Package)
	require.NotNil(t, pkg.Package.TypesInfo)

	inj := pkg.Injectors[0]
	require.NotEmpty(t, inj.Components)

	for _, c := range inj.Components {
		obj := pkg.Package.TypesInfo.Defs[c.Ident]
		require.NotNilf(t, obj, "no type object for component %s", c.Name)
		assert.NotNilf(t, obj.Type(), "no resolved type for component %s", c.Name)
	}
}

// TestOptionsWireArgs asserts each Options field produces exactly the `wire gen`
// flag it mirrors, so runWire's invocation is verifiable without a subprocess.
func TestOptionsWireArgs(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want []string
	}{
		{name: "empty", opts: Options{}, want: nil},
		{
			name: "output file prefix",
			opts: Options{OutputFilePrefix: "foo_"},
			want: []string{"-output_file_prefix=foo_"},
		},
		{
			name: "header file",
			opts: Options{HeaderFile: "header.txt"},
			want: []string{"-header_file=header.txt"},
		},
		{
			name: "tags",
			opts: Options{Tags: "special"},
			want: []string{"-tags=special"},
		},
		{
			name: "all three, in wire's own flag order",
			opts: Options{OutputFilePrefix: "foo_", HeaderFile: "header.txt", Tags: "special"},
			want: []string{"-output_file_prefix=foo_", "-header_file=header.txt", "-tags=special"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.opts.wireArgs())
		})
	}
}

// TestWireDiagnosticsDropsWroteLines asserts Wire's per-output success lines are
// not passed through. Those files are transient and already deleted by the time
// anything is printed, so reporting them would name files Yama had removed.
func TestWireDiagnosticsDropsWroteLines(t *testing.T) {
	logged := strings.Join([]string{
		"wire: example.com/a: wrote /tmp/a/wire_gen.go",
		"wire: example.com/b/wire.go:4:1: inject Init: no provider found for *b.Dep",
		"wire: at least one generate failure",
	}, "\n")

	got := wireDiagnostics(logged, nil)

	assert.NotContains(t, got, "wrote", "a file Yama deleted must not be reported as written")
	assert.Contains(t, got, "no provider found", "the real diagnostic survives")
	assert.Contains(t, got, "at least one generate failure")
}

// TestWireDiagnosticsRenamesDerivedInjectors asserts a diagnostic names the stub
// the application wrote rather than the injector Yama derived from it, and that
// the transient file name keeps the same prefix it carries.
func TestWireDiagnosticsRenamesDerivedInjectors(t *testing.T) {
	const logged = "wire: /tmp/a/yama_wireinject.go:11:1: inject yama_NewLifecycle: no provider found for *a.Dep\n" +
		"wire: /tmp/a/yama_wireinject.go:16:1: inject yama_NewLifecycleWithWriter: no provider found for *a.Dep"

	got := wireDiagnostics(logged, []string{"yama_NewLifecycle", "yama_NewLifecycleWithWriter"})

	assert.Contains(t, got, "inject NewLifecycle:")
	assert.Contains(t, got, "inject NewLifecycleWithWriter:")
	assert.NotContains(t, got, "inject yama_", "no derived name reaches the application")
	assert.Contains(t, got, "yama_wireinject.go", "the transient file name is not a derived injector name")
}

// TestWireDiagnosticsRewritesTheInjectorNameAlone asserts a rewrite reaches the
// name Wire gives an injector, and reaches nothing else in the same diagnostic.
//
// The transient file is named for the build tag it declares. A stub named for
// that tag therefore derives an injector whose name is that file's whole stem,
// and a rewrite of every occurrence would rename the file in the path Wire
// reported.
func TestWireDiagnosticsRewritesTheInjectorNameAlone(t *testing.T) {
	const logged = "wire: /tmp/a/yama_wireinject.go:11:1: inject yama_wireinject: no provider found for *a.Dep"

	got := wireDiagnostics(logged, []string{"yama_wireinject"})

	assert.Contains(t, got, "inject wireinject:", "the injector's name is rewritten")
	assert.Contains(t, got, "/tmp/a/yama_wireinject.go:11:1:", "the path it was reported at is not")
}

// TestEmitHonorsTags asserts Options.Tags reaches every load a run makes, and
// the Wire invocation between them: NewDep is defined only under the "special"
// build tag, so generation must fail without the tag and succeed with it.
//
// The emitted call to NewDep is what proves both ends. Wire resolved the
// provider to write the call, and Yama's own load resolved its type to analyze
// the component the call builds.
func TestEmitHonorsTags(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "tagged")

	_, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"."})
	require.Error(t, err, "without the tag, NewDep does not exist")

	files, err := NewGenerator(Options{Tags: "special"}).EmitAll(ctx, dir, []string{"."})
	require.NoError(t, err)
	require.Len(t, files, 1)

	content := string(files[0].Content)
	assert.Contains(t, content, "func NewLifecycle(")
	assert.Contains(t, content, "NewDep()", "the tagged provider reached Wire and Yama's own load alike")
}

// TestEmitHonorsHeaderFile asserts a valid -header_file does not break
// generation. The header itself is never observed by Yama — wire_gen.go is
// transient — so this is a smoke test that the flag reaches Wire cleanly, not an
// assertion on the header's content.
func TestEmitHonorsHeaderFile(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "tagged")

	files, err := NewGenerator(Options{
		Tags:       "special",
		HeaderFile: "header.txt",
	}).EmitAll(ctx, dir, []string{"."})
	require.NoError(t, err)
	require.Len(t, files, 1)
}

// TestLoadSelectsWireGenAmongGeneratedFiles asserts a load parses Wire's output
// and not Yama's own generated file in the same package. The sandbox fixture
// holds both a wire_gen.go and a lifecycle_gen.go, and only wire_gen.go declares
// the injectors asked for here, so recovering both proves the load selected it.
func TestLoadSelectsWireGenAmongGeneratedFiles(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	want := []string{"InitializeApp", "InitializeAppWithWriter"}

	pkg, err := NewGenerator(Options{}).LoadInjectors(ctx, filepath.Join("testdata", "sandbox"), want)
	require.NoError(t, err)

	var names []string
	for _, inj := range pkg.Injectors {
		names = append(names, inj.Name)
	}
	assert.ElementsMatch(t, want, names)
}

// TestLoadReportsAPackageThatDoesNotTypeCheck asserts a load reports the
// application's own compile error, rather than returning a package whose types
// are missing.
//
// The load that reads a stub already reports one, but it runs under a different
// build tag and does not see every file this one does. Left unreported here, the
// missing types surface later as an *AnalysisError naming a component whose type
// Yama "cannot resolve", which reads as a defect in Yama for an error in the
// application's own source.
func TestLoadReportsAPackageThatDoesNotTypeCheck(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "typeerror")

	_, err := NewGenerator(Options{}).LoadInjectors(context.Background(), dir, []string{"InitializeApp"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "components.go", "the message locates the file that does not compile")
	assert.Contains(t, err.Error(), "not an int", "the message carries what the compiler said")

	var analysisErr *AnalysisError
	assert.False(t, errors.As(err, &analysisErr), "a compile error is not an analysis failure")
}

// TestEmitSkipsAPackageWithNoStub asserts a package that declares no lifecycle
// stub is not a failure. Yama generates for the graphs an application asked to
// orchestrate, and skips the rest in silence, the way Wire itself skips a
// package holding no injector.
func TestEmitSkipsAPackageWithNoStub(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "multipkg", "noinjector")

	files, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"."})
	require.NoError(t, err)
	assert.Empty(t, files)
}

// TestRunWireToolError asserts that a Wire input problem surfaces as a
// *ToolError carrying Wire's own diagnostic, distinct from a toolchain error.
func TestRunWireToolError(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "badwire")

	err := NewGenerator(Options{}).runWire(ctx, dir, []string{"."}, nil)
	require.Error(t, err)

	var toolErr *ToolError
	require.Truef(t, errors.As(err, &toolErr), "want *ToolError, got %T: %v", err, err)

	assert.Contains(t, err.Error(), "wire generation failed")
	assert.Contains(t, err.Error(), "no provider found", "the underlying Wire diagnostic is surfaced")
}

// TestRunWireToolchainError asserts that a failure to launch the tool at all —
// here, a directory that does not exist — surfaces as a *ToolchainError, not a
// *ToolError, so the two causes stay distinct.
func TestRunWireToolchainError(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "does-not-exist")

	err := NewGenerator(Options{}).runWire(ctx, dir, []string{"."}, nil)
	require.Error(t, err)

	var toolchainErr *ToolchainError
	assert.Truef(t, errors.As(err, &toolchainErr), "want *ToolchainError, got %T: %v", err, err)

	var toolErr *ToolError
	assert.Falsef(t, errors.As(err, &toolErr), "must not be a *ToolError: %v", err)
}

// TestNoWireInternalImport verifies, from the dependency graph itself, that the
// generator never reaches into Google Wire's unexported internal packages.
func TestNoWireInternalImport(t *testing.T) {
	requireGo(t)

	cmd := exec.CommandContext(context.Background(), "go", "list", "-deps",
		"l7e.io/yama/v2/internal/generator", "l7e.io/yama/v2/cmd/yama")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	assert.NotContains(t, string(out), "github.com/google/wire/internal",
		"the generator must not depend on Google Wire's internal packages")
}
