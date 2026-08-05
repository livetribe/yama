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

// TestGenerateRunsWireAndResolvesTypes runs Google Wire for real, loads and
// type-checks the package, and asserts that every component identifier the parser
// recorded resolves against the package's type information. This is the
// node-identity contract the model depends on: it type-checks these exact
// nodes rather than reparsing.
func TestGenerateRunsWireAndResolvesTypes(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "structcomp")

	pkg, err := NewGenerator(Options{}).Generate(ctx, dir)
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
	logged := "wire: /tmp/a/yama_wireinject.go:11:1: inject yama_NewLifecycle: no provider found for *a.Dep" +
		"\n" +
		"wire: /tmp/a/yama_wireinject.go:16:1: inject yama_NewLifecycleWithWriter: no provider found for *a.Dep"

	got := wireDiagnostics(logged, []string{"yama_NewLifecycle", "yama_NewLifecycleWithWriter"})

	assert.Contains(t, got, "inject NewLifecycle:")
	assert.Contains(t, got, "inject NewLifecycleWithWriter:")
	assert.NotContains(t, got, "inject yama_", "no derived name reaches the application")
	assert.Contains(t, got, "yama_wireinject.go", "the transient file name is not a derived injector name")
}

// TestGenerateHonorsTags asserts Options.Tags reaches both the Wire invocation
// and Yama's own package load: NewDep is defined only under the "special" build
// tag, so generation must fail without the tag and succeed, with NewDep's type
// resolved, with it.
func TestGenerateHonorsTags(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "tagged")

	_, err := NewGenerator(Options{}).Generate(ctx, dir)
	require.Error(t, err, "without the tag, NewDep does not exist")

	pkg, err := NewGenerator(Options{Tags: "special"}).Generate(ctx, dir)
	require.NoError(t, err)
	require.Len(t, pkg.Injectors, 1)

	inj := pkg.Injectors[0]
	require.NotEmpty(t, inj.Components)

	dep := inj.Components[0]
	assert.Equal(t, "dep", dep.Name)

	obj := pkg.Package.TypesInfo.Defs[dep.Ident]
	require.NotNil(t, obj, "NewDep's result must resolve under the same tag Wire used to generate it")
	assert.Equal(t, "*l7e.io/yama/v2/internal/generator/testdata/tagged.Dep", obj.Type().String())
}

// TestGenerateHonorsHeaderFile asserts a valid -header_file does not break
// generation. The header itself is never observed by Yama — wire_gen.go is
// transient — so this is a smoke test that the flag reaches Wire cleanly, not an
// assertion on the header's content.
func TestGenerateHonorsHeaderFile(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "tagged")

	pkg, err := NewGenerator(Options{
		Tags:       "special",
		HeaderFile: "header.txt",
	}).Generate(ctx, dir)
	require.NoError(t, err)
	require.Len(t, pkg.Injectors, 1)
}

// TestLoadSelectsWireGenAmongGeneratedFiles asserts Load parses Wire's output and
// not Yama's own generated file in the same package. The sandbox fixture holds
// both a wire_gen.go and a lifecycle_gen.go; recovering the injector names Wire
// emits — not the constructor names the lifecycle file declares — proves Load
// selected wire_gen.go.
func TestLoadSelectsWireGenAmongGeneratedFiles(t *testing.T) {
	requireGo(t)

	ctx := context.Background()

	pkg, err := NewGenerator(Options{}).Load(ctx, filepath.Join("testdata", "sandbox"))
	require.NoError(t, err)

	var names []string
	for _, inj := range pkg.Injectors {
		names = append(names, inj.Name)
	}
	assert.ElementsMatch(t, []string{"InitializeApp", "InitializeAppWithWriter"}, names)
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

	_, err := NewGenerator(Options{}).Load(context.Background(), dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "components.go", "the message locates the file that does not compile")
	assert.Contains(t, err.Error(), "not an int", "the message carries what the compiler said")

	var analysisErr *AnalysisError
	assert.False(t, errors.As(err, &analysisErr), "a compile error is not an analysis failure")
}

// TestGenerateNoInjectors asserts that a package with no injector function is
// not a failure: Wire itself silently does nothing there (confirmed against a
// real invocation — exit 0, no file written), and Generate reports the same
// non-error outcome via the ErrNoInjectors sentinel rather than the generic
// file-not-found error Load would otherwise produce.
func TestGenerateNoInjectors(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "multipkg", "noinjector")

	_, err := NewGenerator(Options{}).Generate(ctx, dir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoInjectors), "want ErrNoInjectors, got %v", err)
}

// TestRunWireGenerateError asserts that a Wire input problem surfaces as a
// *GenerateError carrying Wire's own diagnostic, distinct from a toolchain error.
func TestRunWireGenerateError(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "badwire")

	err := NewGenerator(Options{}).runWire(ctx, dir, []string{"."}, nil)
	require.Error(t, err)

	var genErr *GenerateError
	require.Truef(t, errors.As(err, &genErr), "want *GenerateError, got %T: %v", err, err)

	assert.Contains(t, err.Error(), "wire generation failed")
	assert.Contains(t, err.Error(), "no provider found", "the underlying Wire diagnostic is surfaced")
}

// TestRunWireToolchainError asserts that a failure to launch the tool at all —
// here, a directory that does not exist — surfaces as a *ToolchainError, not a
// *GenerateError, so the two causes stay distinct.
func TestRunWireToolchainError(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	dir := filepath.Join("testdata", "does-not-exist")

	err := NewGenerator(Options{}).runWire(ctx, dir, []string{"."}, nil)
	require.Error(t, err)

	var toolErr *ToolchainError
	assert.Truef(t, errors.As(err, &toolErr), "want *ToolchainError, got %T: %v", err, err)

	var genErr *GenerateError
	assert.Falsef(t, errors.As(err, &genErr), "must not be a *GenerateError: %v", err)
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
