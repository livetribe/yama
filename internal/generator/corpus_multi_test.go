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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/wire"
)

// The corpus under internal/generator/testdata holds families of several
// packages. These tests run one whole generation over each family.
//
// A family states its own import paths, so a copy of it has to state the paths
// of the directory that holds the copy.

// corpusPrefix begins the import path of every package in the corpus. A copy
// that these tests make goes beside the family that it copies, so a copy takes
// the same prefix.
const corpusPrefix = "l7e.io/yama/v2/internal/generator/testdata/"

// TestCorpusGeneratesAcrossPackages runs over a family. One package of that
// family builds a graph out of another package's providers. Both packages take
// a lifecycle file, and the package that reads the other one reads what this
// run wrote.
func TestCorpusGeneratesAcrossPackages(t *testing.T) {
	requireGo(t)

	dir := copyFamily(t, "crosspkg")
	t.Chdir(dir)

	d := quietDriver(".", []string{"./..."}, wire.Args{})

	require.NoError(t, d.Run(context.Background()))

	assert.FileExists(t, filepath.Join(dir, "app", "lifecycle_gen.go"))
	assert.FileExists(t, filepath.Join(dir, "lib", "lifecycle_gen.go"))
	assertFamilyBuilds(t)
	assertNoTransient(t, dir)
}

// TestCorpusGeneratesEveryPackageThatGoogleWireTakes runs over a family that
// holds one package that Google Wire rejects. The run reports that rejection,
// and it writes a lifecycle file for every other package.
func TestCorpusGeneratesEveryPackageThatGoogleWireTakes(t *testing.T) {
	requireGo(t)

	dir := copyFamily(t, "multipkg")
	t.Chdir(dir)

	d := quietDriver(".", []string{"./..."}, wire.Args{})

	require.Error(t, d.Run(context.Background()))

	assert.FileExists(t, filepath.Join(dir, "a", "lifecycle_gen.go"))
	assert.FileExists(t, filepath.Join(dir, "b", "lifecycle_gen.go"))
	assert.NoFileExists(t, filepath.Join(dir, "bad", "lifecycle_gen.go"))
	assertFamilyBuilds(t)
	assertNoTransient(t, dir)
}

// TestCorpusReportsAPackageGoogleWireRejects runs over a package with a graph
// that Google Wire cannot build.
func TestCorpusReportsAPackageGoogleWireRejects(t *testing.T) {
	requireGo(t)

	dir := copyFamily(t, "wirefail")
	t.Chdir(dir)

	d := quietDriver(".", []string{"."}, wire.Args{})

	require.Error(t, d.Run(context.Background()))
	assert.NoFileExists(t, filepath.Join(dir, "lifecycle_gen.go"))
	assertNoTransient(t, dir)
}

// TestCorpusGeneratesUnderTheRunsOwnTags runs over a package with providers
// behind a build tag of the application's own.
func TestCorpusGeneratesUnderTheRunsOwnTags(t *testing.T) {
	requireGo(t)

	dir := copyFamily(t, "tagged")
	t.Chdir(dir)

	d := quietDriver(".", []string{"."}, wire.Args{Tags: "special"})

	require.NoError(t, d.Run(context.Background()))
	assert.FileExists(t, filepath.Join(dir, "lifecycle_gen.go"))
	assertNoTransient(t, dir)
}

// TestCorpusLeavesABrokenGraphToTheApplication runs over a package with a graph
// that Google Wire cannot build. The only injector of that package is the
// application's own. The package declares no lifecycle stub, so the run states
// no path for it and Google Wire never reads it. The run reports nothing, and
// it writes nothing.
func TestCorpusLeavesABrokenGraphToTheApplication(t *testing.T) {
	requireGo(t)

	dir := copyFamily(t, "badwire")
	t.Chdir(dir)

	var progress bytes.Buffer

	d := NewDriver(".", []string{"."}, wire.Args{})
	d.progress = &progress

	require.NoError(t, d.Run(context.Background()))
	assert.Empty(t, progress.String())
	assert.NoFileExists(t, filepath.Join(dir, "lifecycle_gen.go"))
	assertNoTransient(t, dir)
}

// sentinelOutput takes the place of Google Wire output that the application
// committed. A run must return the bytes that it found, and not what Google
// Wire wrote over them.
const sentinelOutput = "// sentinel\n\n//go:build !wireinject\n\npackage nogen\n"

// TestCorpusKeepsTheApplicationsOwnWireOutput runs over a package that declares
// no lifecycle stub and builds a graph of its own. Google Wire's output there
// belongs to the application. A run therefore writes no lifecycle file, and it
// returns the output with no change.
func TestCorpusKeepsTheApplicationsOwnWireOutput(t *testing.T) {
	dir := runOverNogen(t, "wire_gen.go", wire.Args{})

	assertSentinel(t, filepath.Join(dir, "wire_gen.go"))
	assertNoYamaTrace(t, dir)
}

// TestCorpusLeavesABackupOfAnEarlierRunAlone runs over the same package. Its
// output is under the backup name, where an interrupted earlier run left it. A
// run takes no custody of a package that declares no stub. The backup stays at
// that name, and the live name stays empty.
func TestCorpusLeavesABackupOfAnEarlierRunAlone(t *testing.T) {
	const backup = ".yama.wire_gen.go"

	dir := runOverNogen(t, backup, wire.Args{})

	assertSentinel(t, filepath.Join(dir, backup))
	assert.NoFileExists(t, filepath.Join(dir, "wire_gen.go"))
}

// TestCorpusKeepsTheApplicationsOwnPrefixedWireOutput runs the same package
// under an output-file prefix. Yama builds the name by Google Wire's own rule.
// The file that Yama sets aside and returns is therefore the file that Google
// Wire writes.
func TestCorpusKeepsTheApplicationsOwnPrefixedWireOutput(t *testing.T) {
	dir := runOverNogen(t, "foo_wire_gen.go", wire.Args{Prefix: "foo_"})

	assertSentinel(t, filepath.Join(dir, "foo_wire_gen.go"))
	assert.NoFileExists(t, filepath.Join(dir, "wire_gen.go"))
	assertNoYamaTrace(t, dir)
}

// runOverNogen puts the sentinel at one name in a copy of the package that
// declares no stub. It then runs over that copy.
func runOverNogen(t *testing.T, start string, args wire.Args) string {
	t.Helper()
	requireGo(t)

	dir := copyFamily(t, "nogen")

	require.NoError(t, os.WriteFile(filepath.Join(dir, start), []byte(sentinelOutput), 0o600))

	t.Chdir(dir)

	d := quietDriver(".", []string{"."}, args)

	require.NoError(t, d.Run(context.Background()))
	assert.NoFileExists(t, filepath.Join(dir, "lifecycle_gen.go"))

	return dir
}

// assertSentinel asserts that the file at path holds the bytes that the run
// found there.
func assertSentinel(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)

	require.NoError(t, err)
	assert.Equal(t, sentinelOutput, string(content))
}

// copyFamily copies one family into a directory of its own. It states the
// import paths of the directory that holds that copy.
func copyFamily(t *testing.T, name string) string {
	t.Helper()

	root, err := filepath.Abs("testdata")
	require.NoError(t, err)

	dir, err := os.MkdirTemp(root, "family-")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	src, err := filepath.Abs(filepath.Join("testdata", name))
	require.NoError(t, err)

	from := corpusPrefix + name + "/"
	to := corpusPrefix + filepath.Base(dir) + "/"

	require.NoError(t, filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return err
		}

		if generated(entry.Name()) {
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		//nolint:gosec // path came from a walk of the corpus, which this file names.
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		stated := strings.ReplaceAll(string(content), from, to)
		target := filepath.Join(dir, rel)

		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}

		//nolint:gosec // target is under the directory that MkdirTemp just made.
		return os.WriteFile(target, []byte(stated), 0o600)
	}))

	return dir
}

// assertFamilyBuilds builds every package of the family that a run settled.
func assertFamilyBuilds(t *testing.T) {
	t.Helper()

	build := exec.CommandContext(context.Background(), "go", "build", "./...")

	out, err := build.CombinedOutput()

	require.NoErrorf(t, err, "go build rejected the family:\n%s", out)
}

// assertNoTransient reports a file that a run writes for Google Wire and takes
// back out. It also reports a backup that a run moves and puts back.
func assertNoTransient(t *testing.T, dir string) {
	t.Helper()

	assertAbsent(t, dir, transient)
}

// assertNoYamaTrace reports a file that Yama itself writes into a target
// package. It states nothing about Google Wire's output. A package that
// declares no lifecycle stub keeps that output.
func assertNoYamaTrace(t *testing.T, dir string) {
	t.Helper()

	assertAbsent(t, dir, yamasOwn)
}

// yamasOwn reports whether a name is one that Yama writes and takes back out:
// a file that a run derives for Google Wire, or a backup that a run moves a
// committed file to.
func yamasOwn(name string) bool {
	return strings.HasPrefix(name, "yama_") || strings.HasPrefix(name, ".yama.")
}

// transient reports a name that a run writes and takes back out: a file that a
// run derives for Google Wire, a backup that a run moves a committed file to,
// and Google Wire's own output.
func transient(name string) bool {
	return yamasOwn(name) || name == wire.BaseOutputName
}

// generated reports every name that a generation puts in a target package: the
// names that a run takes back out, and the lifecycle file that it leaves. A
// copy of a fixture leaves these names behind. A name that reaches the copy
// changes what the copy generates.
//
// No fixture states a lifecycle file of its own. An emit fixture states its
// outcome in a want directory, and a copy never walks into that directory.
func generated(name string) bool {
	return transient(name) || name == wantFile
}

// assertAbsent reports every file under dir with a name that transient accepts.
func assertAbsent(t *testing.T, dir string, transient func(name string) bool) {
	t.Helper()

	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		assert.Falsef(t, transient(entry.Name()), "%s is still in the family", path)

		return nil
	}))
}
