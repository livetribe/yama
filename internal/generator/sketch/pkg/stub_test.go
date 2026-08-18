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

// A package states its stubs one file at a time, in the order of the file names
// and then of the declarations in one file.
func TestLoadReadsEveryStubTheShapesPackageDeclares(t *testing.T) {
	info := loadShapes(t)

	stubs := info.Stubs()

	names := make([]string, 0, len(stubs))
	for i := range stubs {
		names = append(names, stubs[i].Name)
	}

	assert.Equal(t, []string{
		"NewAliased",
		"NewPlain",
		"NewNamedOptions",
		"NewUnnamedOptions",
		"NewBlankOptions",
		"NewNoParams",
	}, names)
}

func TestResultTypeIsTheTypeOfTheValueTheStubBuilds(t *testing.T) {
	stub := shapeStub(t, "NewPlain")

	assert.Equal(t, "*App", stub.ResultType())
}

func TestOptionsNameIsEmptyForAStubThatForwardsNoOptions(t *testing.T) {
	stub := shapeStub(t, "NewPlain")

	assert.False(t, stub.HasOpts)
	assert.Empty(t, stub.OptionsName())
}

func TestOptionsNameIsTheNameTheStubBound(t *testing.T) {
	stub := shapeStub(t, "NewNamedOptions")

	assert.True(t, stub.HasOpts)
	assert.Equal(t, "settings", stub.OptionsName())
}

// A stub can leave the options parameter unnamed. A stub can also bind that
// parameter to the blank identifier. For both, the constructor forwards the
// options under a name of its own.
func TestOptionsNameIsADefaultForAParameterThatBindsNothing(t *testing.T) {
	for _, name := range []string{"NewUnnamedOptions", "NewBlankOptions"} {
		t.Run(name, func(t *testing.T) {
			stub := shapeStub(t, name)

			assert.True(t, stub.HasOpts)
			assert.Equal(t, "opts", stub.OptionsName())
		})
	}
}

func TestGraphParamsDropsTheOptionsParameter(t *testing.T) {
	stub := shapeStub(t, "NewNamedOptions")

	assert.Equal(t, []pkg.Field{{Name: "db", Type: "*sql.DB"}}, stub.GraphParams())
}

func TestGraphParamsKeepsEveryParameterOfAStubThatForwardsNoOptions(t *testing.T) {
	stub := shapeStub(t, "NewPlain")

	assert.Equal(t, []pkg.Field{{Name: "db", Type: "*sql.DB"}}, stub.GraphParams())
}

func TestGraphParamsIsEmptyForAStubThatDeclaresNoParameter(t *testing.T) {
	stub := shapeStub(t, "NewNoParams")

	assert.Empty(t, stub.GraphParams())
}

// An alias holds for one file. The stub takes the name that its own file
// states.
func TestYamaNameAndWireNameTakeTheAliasTheStubsOwnFileStated(t *testing.T) {
	stub := shapeStub(t, "NewAliased")

	assert.Equal(t, "y", stub.YamaName())
	assert.Equal(t, "gwire", stub.WireName())
}

func TestYamaNameAndWireNameTakeTheNameTheOtherFileStates(t *testing.T) {
	stub := shapeStub(t, "NewPlain")

	assert.Equal(t, "yama", stub.YamaName())
	assert.Equal(t, "wire", stub.WireName())
}

func TestLoadReadsTheProvidersTheStubStates(t *testing.T) {
	stub := shapeStub(t, "NewPlain")

	assert.Equal(t, []string{"NewApp"}, stub.Providers)
}

func TestLoadReadsTheDocCommentWithoutItsMarkers(t *testing.T) {
	stub := shapeStub(t, "NewNoParams")

	assert.Equal(t, "NewNoParams declares no parameter at all.", stub.Doc)
}

// The position that a stub carries reaches the derived file, so the Go
// toolchain reports the stub rather than the file that Yama wrote.
func TestLoadReadsThePositionThatDeclaresTheStub(t *testing.T) {
	stub := shapeStub(t, "NewAliased")

	assert.Equal(t, "aliased.go", filepath.Base(stub.File))
	assert.Positive(t, stub.Line)
	assert.Equal(t, 1, stub.Column)
}
