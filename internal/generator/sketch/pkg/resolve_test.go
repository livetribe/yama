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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

func TestNamesReadsTheNameAPackageDeclares(t *testing.T) {
	names := pkg.Names(".", []string{"context"})

	assert.Equal(t, "context", names["context"])
}

// A path that ends in a major version holds no package name. The element before
// it does, and every guess from the path alone has to know that rule.
func TestNamesReadsAPathThatEndsInAMajorVersion(t *testing.T) {
	names := pkg.Names(".", []string{"l7e.io/yama/v2"})

	assert.Equal(t, "yama", names["l7e.io/yama/v2"])
}

// gopkg.in/yaml.v3 declares the package yaml. No rule over the path alone
// produces that name, so a caller that guesses gets it wrong.
func TestNamesReadsAPathThatNoRuleCouldGuess(t *testing.T) {
	names := pkg.Names(".", []string{"gopkg.in/yaml.v3"})

	assert.Equal(t, "yaml", names["gopkg.in/yaml.v3"])
}

func TestNamesReadsEveryPathItWasGiven(t *testing.T) {
	paths := []string{"context", "gopkg.in/yaml.v3", "l7e.io/yama/v2"}

	names := pkg.Names(".", paths)

	assert.Equal(t, map[string]string{
		"context":          "context",
		"gopkg.in/yaml.v3": "yaml",
		"l7e.io/yama/v2":   "yama",
	}, names)
}

func TestNamesReturnsAnEmptyMapForNoPaths(t *testing.T) {
	names := pkg.Names(".", nil)

	assert.Empty(t, names)
}

// A path the toolchain cannot read leaves no entry. The caller keeps whatever
// it already had for that path.
func TestNamesLeavesOutAPathItCannotRead(t *testing.T) {
	names := pkg.Names(".", []string{"example.com/absent"})

	assert.NotContains(t, names, "example.com/absent")
}

func TestImportPathReadsThePathAPackageDeclares(t *testing.T) {
	path := pkg.ImportPath(filepath.Join(stubRoot, "noparams"), []string{pkg.Tag})

	assert.Equal(t, "l7e.io/yama/v2/internal/generator/testdata/stub/noparams", path)
}

func TestImportPathIsEmptyForADirectoryThatHoldsNoPackage(t *testing.T) {
	path := pkg.ImportPath(t.TempDir(), nil)

	assert.Empty(t, path)
}

func TestBuildFlagsSetsNoFlagForNoTags(t *testing.T) {
	assert.Empty(t, pkg.BuildFlags(nil))
}

// Every load that a run makes sets the same tags, and one flag carries them
// all.
func TestBuildFlagsJoinsEveryTagIntoOneFlag(t *testing.T) {
	flags := pkg.BuildFlags([]string{"yamainject", "integration"})

	assert.Equal(t, []string{"-tags=yamainject integration"}, flags)
}
