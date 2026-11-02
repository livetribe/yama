// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/internal/generator/pkg"
)

// The lifecycle file carries the stub files' import block and Google Wire's
// own. One name in that file refers to one path. The check that states this
// rule reads both blocks rather than either one alone.
func TestTakeOutputImportsFromReportsANameThatTwoPathsAnswerTo(t *testing.T) {
	info := pkg.NewInfo(t.TempDir())
	info.TakeStubFile("app", nil, []pkg.Import{{Name: "cfg", Path: "example.com/a/config"}})

	err := info.TakeOutputImportsFrom([]byte("package app\n\nimport cfg \"example.com/b/config\"\n"))

	require.Error(t, err, "the two blocks bind one name to two paths")
	assert.ErrorContains(t, err, "needs an alias")
}

func TestTakeOutputImportsFromTakesOnePathStatedByBothBlocks(t *testing.T) {
	info := pkg.NewInfo(t.TempDir())
	info.TakeStubFile("app", nil, []pkg.Import{{Path: "context"}})

	require.NoError(t, info.TakeOutputImportsFrom([]byte("package app\n\nimport \"context\"\n")))

	assert.Len(t, info.AllImports(), 2, "one entry for each block")
	assert.Len(t, info.LifecycleImports(), 1, "one entry stands for both")
}

func TestTakeOutputImportsFromTakesNothingFromOutputThatDoesNotParse(t *testing.T) {
	info := pkg.NewInfo(t.TempDir())

	require.NoError(t, info.TakeOutputImportsFrom([]byte("this is not go")))
	assert.Empty(t, info.LifecycleImports())
}
