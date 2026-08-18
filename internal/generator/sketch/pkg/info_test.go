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

func TestCollectPackageInfoReadsTheClauseAndThePathOfTheShapesPackage(t *testing.T) {
	info := collectShapes(t)

	assert.Equal(t, "shapes", info.Name())
	assert.Equal(t, "l7e.io/yama/v2/internal/generator/sketch/pkg/testdata/shapes", info.PkgPath())
	assert.Equal(t, shapesDir, info.Dir())
}

// A package states its imports one file at a time. The package holds what every
// file states.
func TestLoadReadsTheImportBlockOfEveryStubFile(t *testing.T) {
	info := loadShapes(t)

	assert.ElementsMatch(t, []pkg.Import{
		{Name: "gwire", Path: pkg.WirePackagePath},
		{Name: "y", Path: pkg.PackagePath},
		{Path: "database/sql"},
		{Path: pkg.WirePackagePath},
		{Name: "yama", Path: pkg.PackagePath},
	}, info.Imports())
}

func TestImportPathsReturnsThePathOfEachImport(t *testing.T) {
	info := loadShapes(t)

	assert.ElementsMatch(t, []string{
		pkg.WirePackagePath,
		pkg.PackagePath,
		"database/sql",
		pkg.WirePackagePath,
		pkg.PackagePath,
	}, info.ImportPaths())
}

// An import that states no alias takes the name that its own package declares.
// Only the Go toolchain states that name.
func TestImportedAsTakesTheNameTheToolchainResolved(t *testing.T) {
	info := collectShapes(t)

	assert.Equal(t, "sql", info.ImportedAs("database/sql"))
}

func TestImportedAsPrefersTheAliasAStubFileStated(t *testing.T) {
	info := collectShapes(t)

	assert.Equal(t, "gwire", info.ImportedAs(pkg.WirePackagePath))
}

func TestImportedAsIsEmptyForAPathNoStubFileImports(t *testing.T) {
	info := collectShapes(t)

	assert.Empty(t, info.ImportedAs("net/http"))
}

// Google Wire states an import block of its own. The package holds only the
// stub files' own block until a run reads that block.
func TestAllImportsHoldsOnlyTheStubFilesOwnBlockAtFirst(t *testing.T) {
	info := loadShapes(t)

	assert.Equal(t, info.Imports(), info.AllImports())
}

// The lifecycle file calls nothing from Google Wire. A stub imports it for its
// Build call only. Each stub file of the shapes package imports it under a name
// of its own.
func TestLifecycleImportsLeavesGoogleWireOutUnderEveryName(t *testing.T) {
	info := collectShapes(t)

	for _, imp := range info.LifecycleImports() {
		assert.NotEqual(t, pkg.WirePackagePath, imp.Path)
	}
}

// A signature can name a package by the alias that one stub file gave it. A
// second signature can name the same package by the name that a second file
// gave. Both names reach the lifecycle file.
func TestLifecycleImportsKeepsOnePathUnderTwoNames(t *testing.T) {
	info := collectShapes(t)

	assert.Contains(t, info.LifecycleImports(), pkg.Import{Name: "y", Path: pkg.PackagePath})
	assert.Contains(t, info.LifecycleImports(), pkg.Import{Name: "yama", Path: pkg.PackagePath})
}

// The tests below drive Info from values rather than from source. The state
// that each one states is a state that no stub package reaches: a package that
// no reader read yet, and a set of names that only two modules with one package
// name would produce.

func TestNewInfoHoldsTheDirectoryItWasGiven(t *testing.T) {
	info := pkg.NewInfo("/some/dir")

	assert.Equal(t, "/some/dir", info.Dir())
	assert.Empty(t, info.Name())
	assert.Empty(t, info.PkgPath())
	assert.Empty(t, info.Stubs())
	assert.Empty(t, info.Imports())
}

// A reader names an import before the Go toolchain resolves anything. The guess
// from the path holds until a resolved name replaces it.
func TestNameOfNamesAnImportBeforeAnyNameIsResolved(t *testing.T) {
	info := pkg.NewInfo("/some/dir")

	name := info.NameOf(pkg.Import{Path: "gopkg.in/yaml.v3"})

	assert.Equal(t, "yaml", name)
}

func TestTakePkgPathStatesTheImportPathThePackageDeclares(t *testing.T) {
	info := pkg.NewInfo("/some/dir")

	info.TakePkgPath("example.com/app")

	assert.Equal(t, "example.com/app", info.PkgPath())
}

// One name in a written file refers to one path. Two paths that resolve to one
// name reach the check that states this rule.
func TestTakeImportNamesReportsANameThatTwoPathsAnswerTo(t *testing.T) {
	info := pkg.NewInfo("/some/dir")
	info.TakeStubFile("app", nil, []pkg.Import{{Path: "example.com/a/config"}, {Path: "example.com/b/config"}})

	err := info.TakeImportNames(map[string]string{
		"example.com/a/config": "config",
		"example.com/b/config": "config",
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "needs an alias")
}

// An alias in the stub file settles a name that two paths would otherwise
// share.
func TestTakeImportNamesTakesTwoConfigPathsUnderAnAlias(t *testing.T) {
	info := pkg.NewInfo("/some/dir")
	info.TakeStubFile("app", nil, []pkg.Import{
		{Path: "example.com/a/config"},
		{Name: "bconfig", Path: "example.com/b/config"},
	})

	err := info.TakeImportNames(map[string]string{
		"example.com/a/config": "config",
		"example.com/b/config": "config",
	})

	require.NoError(t, err)
	assert.Equal(t, "bconfig", info.ImportedAs("example.com/b/config"))
}
