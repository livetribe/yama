package sketch

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// The generator that this sketch replaces is tested against a corpus of target
// packages under internal/generator/testdata. Each emit fixture ships the
// lifecycle file that that generator writes for it.
//
// These tests run the sketch over each fixture and compare what it wrote. The
// corpus states what a generation produces for shapes that no fixture of the
// sketch's own covers, and every fixture in it came from outside the sketch.

// corpusRoot is the directory that holds the fixtures.
const corpusRoot = "../testdata/emit"

// wantFile is the lifecycle file that the fixture states.
const wantFile = "lifecycle_gen.go"

// TestCorpusMatchesTheGeneratorItReplaces renders every fixture and compares it
// with the file that the other generator writes. It then builds the package the
// way an application builds it, which is the fixture's own sources and the file
// that the run wrote.
func TestCorpusMatchesTheGeneratorItReplaces(t *testing.T) {
	requireGo(t)

	for _, name := range corpusFixtures(t) {
		t.Run(name, func(t *testing.T) {
			src, err := filepath.Abs(filepath.Join(corpusRoot, name))
			require.NoError(t, err)

			want, err := os.ReadFile(filepath.Join(src, "want", wantFile))
			require.NoError(t, err)

			got, _ := runCorpusFixture(t, src)

			assert.Truef(t, bytes.Equal(got, want),
				"the fixture states one file and the sketch wrote another\n--- want ---\n%s\n--- got ---\n%s", want, got)

			assertFamilyBuilds(t)
		})
	}
}

// TestCorpusWritesTheSameFileTwice runs over one fixture that a run already
// settled. The second run writes the same bytes as the first.
//
// The cleanup fixture is the one that binds each cleanup to a name of Yama's
// own. That step is the only one that writes over what it read.
func TestCorpusWritesTheSameFileTwice(t *testing.T) {
	requireGo(t)

	src, err := filepath.Abs(filepath.Join(corpusRoot, "cleanup"))
	require.NoError(t, err)

	first, dir := runCorpusFixture(t, src)

	d := quietDriver(dir, []string{"."}, wire.Args{})
	require.NoError(t, d.Run(context.Background()))

	second, err := os.ReadFile(filepath.Join(dir, wantFile))
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
}

// TestCorpusReadsOnlyTheInjectorsItAskedFor runs over a fixture that keeps
// Google Wire injectors of the application's own beside a lifecycle stub.
//
// A build tag keeps the application's wire.go out of every other load, and
// Google Wire copies each declaration of that file into its output. A run reads
// only the injectors it asked Google Wire for. Neither the application's own
// injector nor the declarations beside it reach the lifecycle file.
func TestCorpusReadsOnlyTheInjectorsItAskedFor(t *testing.T) {
	requireGo(t)

	src, err := filepath.Abs(filepath.Join(corpusRoot, "mixed"))
	require.NoError(t, err)

	got, _ := runCorpusFixture(t, src)

	content := string(got)

	assert.Contains(t, content, "func NewLifecycle(")
	assert.NotContains(t, content, "helper")
	assert.NotContains(t, content, "InitializeA")
	assertFamilyBuilds(t)
}

// TestCorpusRunsPastTheFilesAnInterruptedRunLeft runs over a fixture that holds
// both of the files a run derives for Google Wire. A run that ended early
// leaves them, and the run after it writes over each one and takes it out.
func TestCorpusRunsPastTheFilesAnInterruptedRunLeft(t *testing.T) {
	stranded := map[string]string{
		"yama_wireinject.go":  "//go:build wireinject\n\npackage chain\n\nfunc Stranded() {}\n",
		"yama_placeholder.go": "//go:build !yamainject\n\npackage chain\n\nfunc AlsoStranded() {}\n",
	}

	dir := runChainWith(t, stranded)

	assertNoYamaTrace(t, dir)
}

// TestCorpusRunsPastALifecycleFileThatNoLongerCompiles runs over a fixture whose
// committed lifecycle file names a constructor the package no longer declares. A
// rename of a provider leaves the file in that state, and the run that replaces
// it never reads it.
func TestCorpusRunsPastALifecycleFileThatNoLongerCompiles(t *testing.T) {
	stale := map[string]string{
		wantFile: "//go:build !yamainject\n\npackage chain\n\nfunc Stale() *C { return NewRenamedC() }\n",
	}

	dir := runChainWith(t, stale)

	content, err := os.ReadFile(filepath.Join(dir, wantFile))
	require.NoError(t, err)
	assert.Contains(t, string(content), "func NewLifecycle(")

	assertFamilyBuilds(t)
}

// runChainWith copies the chain fixture, puts each named file beside it, and
// runs a whole generation over the result.
func runChainWith(t *testing.T, extra map[string]string) string {
	t.Helper()
	requireGo(t)

	src, err := filepath.Abs(filepath.Join(corpusRoot, "chain"))
	require.NoError(t, err)

	root, err := filepath.Abs("testdata")
	require.NoError(t, err)

	dir, err := os.MkdirTemp(root, "corpus-")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	copyCorpusFiles(t, src, dir)

	for name, content := range extra {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	t.Chdir(dir)

	d := quietDriver(".", []string{"."}, wire.Args{})
	require.NoError(t, d.Run(context.Background()))

	return dir
}

// testappRoot is the one package of the corpus that is an application rather
// than a fixture. It keeps its lifecycle file committed, and a contract test of
// its own runs against that file.
const testappRoot = "../testapp"

// TestCorpusMatchesTheCommittedApplication runs over the application that the
// generator this sketch replaces keeps committed, and it compares the lifecycle
// file with the one that application ships.
func TestCorpusMatchesTheCommittedApplication(t *testing.T) {
	requireGo(t)

	src, err := filepath.Abs(testappRoot)
	require.NoError(t, err)

	want, err := os.ReadFile(filepath.Join(src, wantFile))
	require.NoError(t, err)

	got, _ := runCorpusFixture(t, src)

	assert.Truef(t, bytes.Equal(got, want),
		"the application ships one file and the sketch wrote another\n--- want ---\n%s\n--- got ---\n%s", want, got)

	assertFamilyBuilds(t)
}

// TestSketchTakesNoInternalPackageOfGoogleWire asserts the sketch reaches
// Google Wire through the command alone. No package of the sketch depends on an
// internal package of Google Wire's own.
func TestSketchTakesNoInternalPackageOfGoogleWire(t *testing.T) {
	requireGo(t)

	list := exec.CommandContext(t.Context(), "go", "list", "-deps",
		"l7e.io/yama/v2/internal/generator/sketch")

	out, err := list.CombinedOutput()

	require.NoError(t, err, string(out))
	assert.NotContains(t, string(out), "github.com/google/wire/internal")
}

// corpusFixtures names every fixture that states a lifecycle file of its own.
func corpusFixtures(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(corpusRoot)
	require.NoError(t, err)

	var names []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stated := filepath.Join(corpusRoot, entry.Name(), "want", wantFile)
		if _, err := os.Stat(stated); err != nil {
			continue
		}

		names = append(names, entry.Name())
	}

	require.NotEmpty(t, names)

	return names
}

// runCorpusFixture copies one fixture into a directory of its own and runs a
// whole generation over it. It returns the lifecycle file that the run wrote,
// and the directory that it ran in.
func runCorpusFixture(t *testing.T, src string) (content []byte, dir string) {
	t.Helper()

	root, err := filepath.Abs("testdata")
	require.NoError(t, err)

	dir, err = os.MkdirTemp(root, "corpus-")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	copyCorpusFiles(t, src, dir)
	t.Chdir(dir)

	d := quietDriver(".", []string{"."}, wire.Args{})
	require.NoError(t, d.Run(context.Background()))

	content, err = os.ReadFile(wantFile)
	require.NoError(t, err)

	return content, dir
}

// copyCorpusFiles copies the Go files of one fixture. It leaves the want
// directory behind, which states the outcome rather than taking part in it.
func copyCorpusFiles(t *testing.T, src, dir string) {
	t.Helper()

	entries, err := os.ReadDir(src)
	require.NoError(t, err)

	for _, entry := range entries {
		name := filepath.Base(entry.Name())
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || transient(name) {
			continue
		}

		content, err := os.ReadFile(filepath.Join(src, name))
		require.NoError(t, err)

		//nolint:gosec // the name comes from a directory listing of the fixture, and dir is one MkdirTemp just made.
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
	}
}
