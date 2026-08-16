package sketch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// writeHeader puts content at name in dir and returns the path it wrote.
func writeHeader(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// A Driver writes to os.Stderr, and no caller of NewDriver hands it a stream.
// The command that runs a generation reads this default rather than setting one,
// so a run reports what it wrote only while the default stands.
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

func TestDriverHeaderKeepsTheBytesItRead(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "boilerplate.txt", "// One.\n// Two.")

	d := NewDriver(dir, nil, wire.Args{HeaderFile: "boilerplate.txt"})

	header, err := d.header()

	require.NoError(t, err)
	assert.Equal(t, "// One.\n// Two.", string(header))
}
