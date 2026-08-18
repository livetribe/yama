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

package resolve_test

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/pkg"
	"l7e.io/yama/v2/internal/generator/resolve"
)

// herePath is the import path of the resolve package itself, which is the
// package that these tests run in.
const herePath = "l7e.io/yama/v2/internal/generator/resolve"

// here is the directory that these tests run in, which is the directory of the
// resolve package itself.
func here(t *testing.T) string {
	t.Helper()

	dir, err := filepath.Abs(".")
	require.NoError(t, err)

	return dir
}

func TestPackagesTakesNoPatternToMeanTheCurrentDirectory(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), ".", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{here(t)}, dirs)
}

func TestPackagesResolvesOneDirectory(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), ".", nil, []string{"."})

	require.NoError(t, err)
	assert.Equal(t, []string{here(t)}, dirs)
}

// "./..." is the pattern that a run over a whole tree uses. It is also the
// pattern that a caller cannot expand for itself.
func TestPackagesExpandsARecursivePattern(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), "..", nil, []string{"./..."})

	require.NoError(t, err)
	assert.Greater(t, len(dirs), 1)
	assert.Contains(t, dirs, here(t))
}

func TestPackagesReturnsTheDirectoriesSorted(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), "..", nil, []string{"./..."})

	require.NoError(t, err)
	assert.True(t, sort.StringsAreSorted(dirs))
}

func TestPackagesReturnsNoDirectoryTwice(t *testing.T) {
	patterns := []string{".", ".", "./..."}

	dirs, err := resolve.Packages(context.Background(), ".", nil, patterns)

	require.NoError(t, err)

	unique := slices.Clone(dirs)
	unique = slices.Compact(unique)

	assert.Equal(t, unique, dirs)
}

// testdata holds no package that a pattern matches. The Go toolchain leaves it
// out of every wildcard. This is what keeps a fixture out of a run.
func TestPackagesLeavesOutTestdata(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), "..", nil, []string{"./..."})

	require.NoError(t, err)

	for _, dir := range dirs {
		assert.NotContains(t, dir, "testdata")
	}
}

func TestPackagesReportsAPatternItCannotRead(t *testing.T) {
	_, err := resolve.Packages(context.Background(), ".", nil, []string{"./absent"})

	require.Error(t, err)
}

// A tag changes which packages a pattern matches. Resolution sets the same tags
// that Google Wire receives, so both reach the same set.
func TestPackagesResolvesUnderTheTagsItWasGiven(t *testing.T) {
	dirs, err := resolve.Packages(context.Background(), ".", []string{"yamainject"}, []string{"."})

	require.NoError(t, err)
	assert.Equal(t, []string{here(t)}, dirs)
}

// A run names each package that it wrote a file for. It uses the import path
// that the package declares.
func TestPackagePathNamesThePackageInTheDirectory(t *testing.T) {
	assert.Equal(t, herePath, pkg.ImportPath(here(t), nil))
}

// A directory that holds no package states no import path. That is not a
// failure. A run still writes the file that it generated.
func TestPackagePathTakesADirectoryThatHoldsNoPackage(t *testing.T) {
	assert.Empty(t, pkg.ImportPath(t.TempDir(), nil))
}
