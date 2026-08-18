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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/wire"
)

// writeHeader puts content at name in dir. It returns the path that it wrote.
func writeHeader(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// A Driver writes to os.Stderr, and no caller of NewDriver gives it a stream.
// The command that runs a generation reads this default. It sets no stream of
// its own, so a run reports what it wrote only while the default holds.
func TestDriverWritesToStandardErrorByDefault(t *testing.T) {
	d := NewDriver(t.TempDir(), nil, wire.Args{})

	assert.Same(t, os.Stderr, d.progress)
}

func TestDriverHeaderReadsNothingWhenTheRunNamedNoFile(t *testing.T) {
	d := NewDriver(t.TempDir(), nil, wire.Args{})

	header, err := d.header()

	require.NoError(t, err)
	assert.Nil(t, header)
}

func TestDriverHeaderResolvesARelativeNameAgainstTheRunDirectory(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "boilerplate.txt", "// Copyright the authors.\n")

	d := NewDriver(dir, nil, wire.Args{HeaderFile: "boilerplate.txt"})

	header, err := d.header()

	require.NoError(t, err)
	assert.Equal(t, "// Copyright the authors.\n", string(header))
}

func TestDriverHeaderReadsAnAbsolutePathAsItStands(t *testing.T) {
	path := writeHeader(t, t.TempDir(), "boilerplate.txt", "// Copyright the authors.\n")

	d := NewDriver(t.TempDir(), nil, wire.Args{HeaderFile: path})

	header, err := d.header()

	require.NoError(t, err)
	assert.Equal(t, "// Copyright the authors.\n", string(header))
}

func TestDriverHeaderReportsAFileItCannotRead(t *testing.T) {
	d := NewDriver(t.TempDir(), nil, wire.Args{HeaderFile: "absent.txt"})

	_, err := d.header()

	require.Error(t, err)
	assert.ErrorContains(t, err, "absent.txt")
}

// Every file that a run writes puts the header above its own package clause, so
// a header holds comments and blank lines only. A run reports a header that
// holds something else. It does not fail to render.
func TestDriverHeaderReportsAFileThatNoGoFileCanCarry(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "NOTICE", "Copyright 2026 the authors.\n")

	d := NewDriver(dir, nil, wire.Args{HeaderFile: "NOTICE"})

	_, err := d.header()

	require.Error(t, err)
	assert.ErrorContains(t, err, "NOTICE")
	assert.ErrorContains(t, err, "not a comment")
}

func TestDriverHeaderTakesABuildConstraint(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "boilerplate.txt", "// Copyright.\n//\n//go:build !ignore\n")

	d := NewDriver(dir, nil, wire.Args{HeaderFile: "boilerplate.txt"})

	_, err := d.header()

	require.NoError(t, err)
}

func TestDriverHeaderKeepsTheBytesItRead(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "boilerplate.txt", "// One.\n// Two.")

	d := NewDriver(dir, nil, wire.Args{HeaderFile: "boilerplate.txt"})

	header, err := d.header()

	require.NoError(t, err)
	assert.Equal(t, "// One.\n// Two.", string(header))
}
