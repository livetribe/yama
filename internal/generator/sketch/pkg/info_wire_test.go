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

package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
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
