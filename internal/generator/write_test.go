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
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteCommitsTheContent asserts a write lands the content at the expected
// path.
func TestWriteCommitsTheContent(t *testing.T) {
	dir := t.TempDir()
	file := &LifecycleFile{Dir: dir, Name: lifecycleGenName, Content: []byte("package p\n")}

	path, err := file.Write()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, lifecycleGenName), path)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "package p\n", string(got))
}

// TestWriteKeepsTheCommittedFileMode asserts a lifecycle file already in the
// package keeps the permission it carries. An application that set a mode on its
// generated file deliberately still has that mode after it regenerates.
func TestWriteKeepsTheCommittedFileMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, lifecycleGenName)
	require.NoError(t, os.WriteFile(target, []byte("package stale\n"), 0o600))

	file := &LifecycleFile{Dir: dir, Name: lifecycleGenName, Content: []byte("package fresh\n")}

	_, err := file.Write()
	require.NoError(t, err)

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
}

// TestWriteReplacesTheCommittedFile asserts a second write replaces the first's
// content rather than appending to it or failing on the existing file.
func TestWriteReplacesTheCommittedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, lifecycleGenName)
	require.NoError(t, os.WriteFile(target, []byte("package stale\n"), lifecycleFileMode))

	file := &LifecycleFile{Dir: dir, Name: lifecycleGenName, Content: []byte("package fresh\n")}

	_, err := file.Write()
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "package fresh\n", string(got))
}

// TestWriteFailsWithoutTouchingTheCommittedFile asserts a write the file system
// refuses reports the failure and leaves the previous file whole. The write is
// refused as the file opens, before anything truncates it, so the package still
// compiles against the file it already had.
func TestWriteFailsWithoutTouchingTheCommittedFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser writes to a read-only file, so the failure cannot be staged")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, lifecycleGenName)
	require.NoError(t, os.WriteFile(target, []byte("package committed\n"), 0o600))
	require.NoError(t, os.Chmod(target, 0o444))

	file := &LifecycleFile{Dir: dir, Name: lifecycleGenName, Content: []byte("package fresh\n")}

	_, err := file.Write()
	require.Error(t, err)
	assert.Contains(t, err.Error(), lifecycleGenName, "the diagnostic names the file it could not write")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "package committed\n", string(got))
}
