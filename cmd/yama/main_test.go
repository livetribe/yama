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
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator"
)

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
			parsed, err := parseArgs(tc.args)
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
// rather than silently swallowed or treated as a package pattern.
func TestParseArgsRejectsUnknownFlag(t *testing.T) {
	_, err := parseArgs([]string{"-no_such_flag"})
	require.Error(t, err)
}

// TestRunThreadsFlagsToGeneration is an end-to-end smoke test that flags
// parsed by the command actually reach generation: it points -tags at
// testdata/tagged, whose sole dependency exists only under that tag, so the run
// only succeeds if the flag was threaded all the way through. This fixture is
// dedicated to cmd/yama, kept separate from internal/generator's own fixture of
// the same shape so the two packages' tests never touch the same directory —
// wireGenScope's exclusive claim would otherwise correctly, but flakily, reject
// whichever test lost the race when `go test ./...` runs packages concurrently.
func TestRunThreadsFlagsToGeneration(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not found on PATH: %v", err)
	}

	// The leading "./" is required: like `wire gen` itself, a bare relative
	// pattern is parsed as an import path, not a filesystem path (confirmed
	// against a real `wire gen testdata/tagged` invocation, which fails
	// identically — "package testdata/tagged is not in std").
	pattern := "./" + filepath.Join("testdata", "tagged")
	ctx := context.Background()

	err := run(ctx, []string{"-tags=special", pattern})
	require.NoError(t, err)
}
