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
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator"
)

// requireGo skips a test that needs the go command, which the command shells out
// to for Google Wire.
func requireGo(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not found on PATH: %v", err)
	}
}

// dirNames is the names of dir's entries, sorted, as os.ReadDir returns them.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}

	return names
}

// TestParseArgs asserts each flag yama accepts matches `wire gen`'s own flag
// name and default, and that package patterns are collected as positional
// arguments regardless of how many are given.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantOpts     generator.Options
		wantPatterns []string
	}{
		{
			name:         "no arguments",
			args:         nil,
			wantOpts:     generator.Options{},
			wantPatterns: nil,
		},
		{
			name:         "output_file_prefix",
			args:         []string{"-output_file_prefix=foo_"},
			wantOpts:     generator.Options{OutputFilePrefix: "foo_"},
			wantPatterns: nil,
		},
		{
			name:         "header_file",
			args:         []string{"-header_file=header.txt"},
			wantOpts:     generator.Options{HeaderFile: "header.txt"},
			wantPatterns: nil,
		},
		{
			name:         "tags",
			args:         []string{"-tags=special"},
			wantOpts:     generator.Options{Tags: "special"},
			wantPatterns: nil,
		},
		{
			name: "all three flags together",
			args: []string{"-output_file_prefix=foo_", "-header_file=header.txt", "-tags=special"},
			wantOpts: generator.Options{
				OutputFilePrefix: "foo_",
				HeaderFile:       "header.txt",
				Tags:             "special",
			},
			wantPatterns: nil,
		},
		{
			name:         "a single package pattern",
			args:         []string{"./..."},
			wantOpts:     generator.Options{},
			wantPatterns: []string{"./..."},
		},
		{
			name:         "multiple package patterns",
			args:         []string{"./a", "./b"},
			wantOpts:     generator.Options{},
			wantPatterns: []string{"./a", "./b"},
		},
		{
			name:         "flags before patterns, matching wire's own usage",
			args:         []string{"-tags=special", "./a", "./b"},
			wantOpts:     generator.Options{Tags: "special"},
			wantPatterns: []string{"./a", "./b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parseArgs(io.Discard, tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.wantOpts, parsed.opts)

			// flag.FlagSet.Args returns an empty, non-nil slice when nothing is
			// left over, so an expected nil and an expected empty slice are
			// asserted the same way here.
			if len(tc.wantPatterns) == 0 {
				assert.Empty(t, parsed.patterns)
			} else {
				assert.Equal(t, tc.wantPatterns, parsed.patterns)
			}
		})
	}
}

// TestParseArgsRejectsUnknownFlag asserts an unrecognized flag is reported
// rather than silently swallowed or treated as a package pattern, that the
// diagnostic and the usage text reach the writer the caller named, and that the
// error arrives as a *usageError so the command exits with the usage code.
func TestParseArgsRejectsUnknownFlag(t *testing.T) {
	var reported bytes.Buffer

	_, err := parseArgs(&reported, []string{"-no_such_flag"})
	require.Error(t, err)

	var usage *usageError
	assert.ErrorAs(t, err, &usage)
	assert.Contains(t, reported.String(), "-no_such_flag")
	assert.Contains(t, reported.String(), "Usage: yama")
}

// taggedPattern is the package pattern naming the CLI's tagged fixture.
//
// The leading "./" is required: like `wire gen` itself, a bare relative pattern
// is parsed as an import path, not a filesystem path (confirmed against a real
// `wire gen testdata/tagged` invocation, which fails identically — "package
// testdata/tagged is not in std").
var taggedPattern = "./" + filepath.ToSlash(filepath.Join("testdata", "tagged"))

// TestRunGeneratesTheCommittedFile is the command's end-to-end check. It points
// -tags at testdata/tagged, whose sole dependency exists only under that tag, so
// the run only succeeds if the flag was threaded all the way through. It then
// asserts what the command leaves behind: the committed lifecycle file, the
// progress line naming it, and nothing else.
//
// This fixture is dedicated to cmd/yama, kept separate from internal/generator's
// own fixture of the same shape so the two packages' tests never touch the same
// directory — wireGenScope's exclusive claim would otherwise correctly, but
// flakily, reject whichever test lost the race when `go test ./...` runs
// packages concurrently.
func TestRunGeneratesTheCommittedFile(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "tagged")
	target := filepath.Join(dir, "lifecycle_gen.go")
	t.Cleanup(func() {
		_ = os.Remove(target)
	})

	var progress bytes.Buffer
	require.NoError(t, run(t.Context(), &progress, []string{"-tags=special", taggedPattern}))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(content), "// Code generated by Yama. DO NOT EDIT.")

	absTarget, err := filepath.Abs(target)
	require.NoError(t, err)
	assert.Equal(t,
		"yama: l7e.io/yama/v2/cmd/yama/testdata/tagged: wrote "+absTarget+"\n",
		progress.String(),
		"the progress line names Yama's artifact in the shape Wire names its own")

	// Everything generation writes apart from the lifecycle file is transient:
	// Wire's output, the derived injector, the backups holding the originals of
	// all three, and the file a write is staged in.
	assert.ElementsMatch(t,
		[]string{"components.go", "dep_special.go", "lifecycle.go", "lifecycle_gen.go"},
		dirNames(t, dir))
}

// TestRunIsIdempotent asserts a second run over an unchanged package leaves the
// committed file byte-identical, which is what makes the drift check a check of
// the inputs rather than of the run.
func TestRunIsIdempotent(t *testing.T) {
	requireGo(t)

	target := filepath.Join("testdata", "tagged", "lifecycle_gen.go")
	t.Cleanup(func() {
		_ = os.Remove(target)
	})

	args := []string{"-tags=special", taggedPattern}

	require.NoError(t, run(t.Context(), io.Discard, args))
	first, err := os.ReadFile(target)
	require.NoError(t, err)

	require.NoError(t, run(t.Context(), io.Discard, args))
	second, err := os.ReadFile(target)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
}

// TestRunSkipsAPackageWithNoStub asserts a named package that declares no
// lifecycle stub succeeds, writes nothing, and prints nothing — the behavior
// `wire gen` has for a package with no injector.
func TestRunSkipsAPackageWithNoStub(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "nostubs")
	pattern := "./" + filepath.ToSlash(dir)

	var progress bytes.Buffer
	require.NoError(t, run(t.Context(), &progress, []string{pattern}))

	assert.Empty(t, progress.String())
	assert.Equal(t, []string{"components.go"}, dirNames(t, dir))
}

// TestWriteFilesReportsADefectAWriteFailureHides asserts a rendering defect
// reaches the caller even when the write of that same file fails.
//
// The two arrive by different routes — the defect on the emitted file, the
// failure from the write — and reporting only the second would leave the
// malformed content undiagnosed until the write succeeded.
func TestWriteFilesReportsADefectAWriteFailureHides(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser writes to a read-only directory, so the failure cannot be staged")
	}

	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	defect := errors.New("the emitter built source Go cannot format")
	file := &generator.LifecycleFile{
		Dir:     dir,
		PkgPath: "example.com/p",
		Name:    "lifecycle_gen.go",
		Content: []byte("package p\n"),
		Errs:    []error{defect},
	}

	err := writeFiles(io.Discard, []*generator.LifecycleFile{file})
	require.Error(t, err)
	assert.ErrorIs(t, err, defect)
}

// TestRunRejectsAPatternThatMatchesNothing asserts a pattern resolving to no
// package fails the run rather than passing over it, matching `wire gen`'s own
// exit-1 diagnostic for the same input.
func TestRunRejectsAPatternThatMatchesNothing(t *testing.T) {
	requireGo(t)

	err := run(t.Context(), io.Discard, []string{"./testdata/no_such_package"})
	require.Error(t, err)
}
