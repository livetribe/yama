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
	"flag"
	"go/format"
	"os"
	"os/exec"

	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// update records the emitted file as the new golden instead of comparing
// against the committed one. A golden diff is coupled to the runtime API, to
// the parser, and to the analysis, so any change to one of them produces one.
// The diff is the record of what the generated file now promises: read it and
// confirm the change was intended before running with this flag.
var update = flag.Bool("update", false, "record the emitted lifecycle files as the goldens")

// goldenDirName holds one fixture's expected output, following Google Wire's own
// test layout.
const goldenDirName = "want"

// goldenFileMode is the permission a recorded golden and a materialized source
// file are written with.
const goldenFileMode = 0o600

// fixtureSources are the files every fixture package in the corpus is written
// from. The stub is copied with the components, so a build that succeeds also
// shows the stub's build tag keeping its duplicate declaration out.
var fixtureSources = []string{"components.go", "lifecycle.go"}

// emitCases are the fixture packages the golden corpus covers. Each is a
// self-contained package with a lifecycle stub and a want/ directory.
var emitCases = []string{
	"blankopts",
	"capabilities",
	"chain",
	"cleanup",
	"collision",
	"diamond",
	"empty",
	"fanout",
	"noopts",
	"optscollision",
	"optsshadow",
	"pkgscope",
	"single",
	"variadicgraphparam",
	"versioned",
}

// TestEmitGoldens generates each fixture package's lifecycle file and compares
// it against the committed golden.
func TestEmitGoldens(t *testing.T) {
	requireGo(t)

	for _, name := range emitCases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "emit", name)
			file := emitFixture(t, dir)

			golden := filepath.Join(dir, goldenDirName, lifecycleGenName)
			if *update {
				require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
				require.NoError(t, os.WriteFile(golden, file.Content, goldenFileMode))

				return
			}

			want, err := os.ReadFile(golden)
			require.NoError(t, err, "no golden recorded; run go test -run TestEmitGoldens -update ./internal/generator")
			assert.Equal(t, string(want), string(file.Content))
		})
	}
}

// TestEmitTestAppMatchesCommitted asserts the lifecycle file committed for the
// behavioral fixture is the one the generator emits for it today.
//
// That fixture's committed file is what the contract suite runs, so a change to
// the emitter that the suite would otherwise never see fails here instead.
func TestEmitTestAppMatchesCommitted(t *testing.T) {
	requireGo(t)

	file := emitFixture(t, "testapp")

	committed := filepath.Join("testapp", lifecycleGenName)
	if *update {
		require.NoError(t, os.WriteFile(committed, file.Content, goldenFileMode))

		return
	}

	want, err := os.ReadFile(committed)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file.Content))
}

// TestEmitIgnoresTheApplicationsOwnWireFile asserts a package that keeps its own
// Wire injectors still generates.
//
// Google Wire copies the non-injector declarations of a wire.go into its output,
// since that file is tagged out of the normal build. Reading every function in
// that output as an injector fails on the first one that is not, so Yama reads
// only the injectors it asked Wire to generate.
func TestEmitIgnoresTheApplicationsOwnWireFile(t *testing.T) {
	requireGo(t)

	file := emitFixture(t, filepath.Join("testdata", "emit", "mixed"))

	content := string(file.Content)
	assert.Contains(t, content, "func NewLifecycle(")
	assert.NotContains(t, content, "helper", "the application's own declarations are not read")
	assert.NotContains(t, content, "InitializeA", "the application's own injector gets no constructor")
}

// TestEmittedFileCarriesItsHeaderAndTag asserts the two things the emitted file
// must open with: the provenance header the Go ecosystem recognizes, and the
// build constraint that keeps the file out of the load that reads the stubs
// declaring the same constructors.
func TestEmittedFileCarriesItsHeaderAndTag(t *testing.T) {
	requireGo(t)

	file := emitFixture(t, filepath.Join("testdata", "emit", "chain"))

	lines := strings.Split(string(file.Content), "\n")
	require.Greater(t, len(lines), 3)
	assert.Equal(t, generatedHeader, lines[0])
	assert.Equal(t, "//go:build !yamainject", lines[2])
}

// TestEmittedFileIsGofmtClean asserts that re-formatting the emitted file is a
// no-op, so a committed lifecycle file never shows up as a formatting diff.
func TestEmittedFileIsGofmtClean(t *testing.T) {
	requireGo(t)

	for _, name := range emitCases {
		t.Run(name, func(t *testing.T) {
			file := emitFixture(t, filepath.Join("testdata", "emit", name))

			formatted, err := format.Source(file.Content)
			require.NoError(t, err)
			assert.Equal(t, string(file.Content), string(formatted))
		})
	}
}

// TestEmitIsRepeatable asserts a second run over an unchanged package emits
// byte-identical content. The cleanup fixture is the one that exercises the
// rebinding, which is the only step that edits what it reads.
func TestEmitIsRepeatable(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "cleanup")

	first := emitFixture(t, dir)
	second := emitFixture(t, dir)

	assert.Equal(t, string(first.Content), string(second.Content))
}

// TestEmittedPackageCompiles builds each fixture package as an application
// would: its own sources plus the emitted lifecycle file, with the stub excluded
// by its build tag.
func TestEmittedPackageCompiles(t *testing.T) {
	requireGo(t)

	for _, name := range emitCases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "emit", name)
			file := emitFixture(t, dir)

			built := materialize(t, dir, name, file.Content)

			cmd := exec.CommandContext(t.Context(), "go", "build", built)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "go build %s:\n%s", built, out)
		})
	}
}

// emitFixture runs the whole pipeline over one fixture package and returns the
// single lifecycle file it produced.
func emitFixture(t *testing.T, dir string) *LifecycleFile {
	t.Helper()

	files, err := NewGenerator(Options{}).EmitAll(context.Background(), dir, []string{"."})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Empty(t, files[0].Errs, "rendering reported a defect in the emitter")

	return files[0]
}

// materialize writes a fixture's own sources and its emitted lifecycle file into
// a scratch package inside the module, and returns the pattern that builds it.
//
// The package must sit under the module for its import of the public package to
// resolve, and outside the fixture directory so a build leaves the corpus as it
// found it.
func materialize(t *testing.T, dir, name string, emitted []byte) string {
	t.Helper()

	root := filepath.Join("testdata", "emitbuild")
	target := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(target, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(target)
	})

	for _, name := range fixtureSources {
		content, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		//nolint:gosec // both path elements are constants of this test file.
		require.NoError(t, os.WriteFile(filepath.Join(target, name), content, goldenFileMode))
	}
	require.NoError(t, os.WriteFile(filepath.Join(target, lifecycleGenName), emitted, goldenFileMode))

	return "./" + filepath.ToSlash(target)
}
