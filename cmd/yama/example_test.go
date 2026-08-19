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

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exampleDir is the example application, relative to this package. It is its own
// Go module, so what it builds against reaches it as an ordinary external
// dependency.
//
// Its module path must stay outside l7e.io/yama/v2/. Go's internal-package rule
// is keyed on the import-path prefix, not on the module: a module path under
// l7e.io/yama/v2/ may import l7e.io/yama/v2/internal/..., and the boundary these
// tests prove is then no longer under test.
var exampleDir = filepath.Join("..", "..", "examples", "hello")

// exampleGenName is the one file the example commits from generation.
const exampleGenName = "lifecycle_gen.go"

// TestExampleBuildsAsItsOwnModule asserts the example compiles and vets as a
// standalone module. The generated file reaches Yama across a module boundary
// there, so an import of a Yama internal package fails this with Go's own
// internal-visibility error.
func TestExampleBuildsAsItsOwnModule(t *testing.T) {
	requireGo(t)

	runInExample(t, "build", "./...")
	runInExample(t, "vet", "./...")
}

// TestExampleResolvesTheLocalCheckout asserts the example builds against this
// working tree. Without it the build could satisfy every other check here
// against a published version, and prove nothing about the change under test.
func TestExampleResolvesTheLocalCheckout(t *testing.T) {
	requireGo(t)

	resolved := runInExample(t, "list", "-m", "-f", "{{.Dir}}", "l7e.io/yama/v2")

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	assert.Equal(t, realPath(t, root), realPath(t, strings.TrimSpace(resolved)))
}

// TestExampleBreaksWhenTheLocalAPIBreaks is the change-detection half of the
// proof above. It builds the example over an overlay that renames an exported
// function the generated file calls, and asserts the build fails on it.
//
// A build that still succeeds means the example is reading some other copy of
// Yama, and every check here is measuring that copy instead.
func TestExampleBreaksWhenTheLocalAPIBreaks(t *testing.T) {
	requireGo(t)

	overlay := renameBuilderOverlay(t)

	//nolint:gosec // the overlay path is a file this test wrote under its own temporary directory.
	cmd := exec.CommandContext(t.Context(), "go", "build", "-overlay="+overlay, "./...")
	cmd.Dir = exampleDir

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "the example built against a Yama whose builder no longer exists:\n%s", out)
	assert.Contains(t, string(out), "NewLifecycleBuilder",
		"the failure names the renamed function the generated file calls")
}

// TestExampleGenerationMatchesTheCommittedFile asserts `go generate ./...` in
// the example reproduces the file the example commits, and that a second run
// reproduces it again.
//
// It restores the committed file afterward, so a run that does not reproduce it
// reports the difference rather than leaving it in the working tree.
func TestExampleGenerationMatchesTheCommittedFile(t *testing.T) {
	requireGo(t)

	target := filepath.Join(exampleDir, exampleGenName)

	committed, err := os.ReadFile(target)
	require.NoError(t, err)
	t.Cleanup(func() {
		//nolint:gosec // the mode is the one a committed source file carries, which this restores.
		_ = os.WriteFile(target, committed, 0o644)
	})

	runInExample(t, "generate", "./...")
	first, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, string(committed), string(first), "generation drifted from the committed file")

	runInExample(t, "generate", "./...")
	second, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "a second run changed the file the first one wrote")
}

// TestExampleLifecycleTestsPass runs the example module's own tests. Those
// tests start the generated lifecycle, assert the order of its component
// calls, and assert its startup error contract. The inner run adds -race
// only when this binary has the race detector.
func TestExampleLifecycleTestsPass(t *testing.T) {
	requireGo(t)

	trackExampleSources(t)

	args := []string{"test"}
	if raceDetectorEnabled {
		args = append(args, "-race")
	}
	args = append(args, "./...")

	runInExample(t, args...)
}

// runInExample runs the go command in the example module and returns its
// combined output, failing the test if the command does.
func runInExample(t *testing.T, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = exampleDir

	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "go %s:\n%s", strings.Join(args, " "), out)

	return string(out)
}

// renameBuilderOverlay writes an overlay that renames the runtime-support
// package's builder constructor, and returns the overlay file's path.
//
// The replacement is the checked-out source with the one name changed, so the
// overlay carries no second copy of the file to keep current.
func renameBuilderOverlay(t *testing.T) string {
	t.Helper()

	builder, err := filepath.Abs(filepath.Join("..", "..", "rt", "builder.go"))
	require.NoError(t, err)

	source, err := os.ReadFile(builder)
	require.NoError(t, err)

	renamed := strings.Replace(string(source), "func NewLifecycleBuilder(", "func NewLifecycleBuilderRenamed(", 1)
	require.NotEqual(t, string(source), renamed, "the builder constructor was not found to rename")

	scratch := t.TempDir()
	replacement := filepath.Join(scratch, "builder.go")
	//nolint:gosec // the path is this test's own temporary directory.
	require.NoError(t, os.WriteFile(replacement, []byte(renamed), 0o600))

	content, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: map[string]string{builder: replacement}})
	require.NoError(t, err)

	overlay := filepath.Join(scratch, "overlay.json")
	require.NoError(t, os.WriteFile(overlay, content, 0o600))

	return overlay
}

// realPath resolves every symbolic link in path, so two spellings of one
// directory compare equal.
func realPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)

	return resolved
}

// trackExampleSources reads every Go source file and both module files in the
// example module. The Go test cache records the reads. A later change to one
// of those files makes this package's cached test result stale.
func trackExampleSources(t *testing.T) {
	t.Helper()

	patterns := []string{"*.go", "go.mod", "go.sum", filepath.Join("cmd", "hello", "*.go")}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(exampleDir, pattern))
		require.NoError(t, err)

		for _, match := range matches {
			_, err := os.ReadFile(match)
			require.NoError(t, err)
		}
	}
}
