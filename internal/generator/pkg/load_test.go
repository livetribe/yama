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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/pkg"
)

// loadStub writes one stub file and reads the package that it declares.
func loadStub(t *testing.T, body string) *pkg.Info {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lifecycle.go"), []byte(body), 0o600))

	info, err := pkg.Load(dir, nil)
	require.NoError(t, err)
	require.Len(t, info.Stubs(), 1)

	return info
}

// A stub forwards Yama's own options. It forwards no other package's options. A
// variadic parameter of any other Option type is an ordinary graph parameter.
func TestLoadReadsAVariadicOptionOfAnotherPackageAsAGraphParameter(t *testing.T) {
	info := loadStub(t, `//go:build yamainject

package app

import (
	"github.com/google/wire"
	yama "l7e.io/yama/v2"
	"go.uber.org/zap"
)

func NewApp(logOpts ...zap.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewA))
}
`)

	stub := info.Stubs()[0]

	assert.False(t, stub.HasOpts, "zap.Option is not Yama's Option")
	assert.Len(t, stub.GraphParams(), 1, "the parameter stays in the graph")
	assert.Empty(t, stub.OptionsName())
}

func TestLoadReadsYamasOwnVariadicOptionUnderAnAlias(t *testing.T) {
	info := loadStub(t, `//go:build yamainject

package app

import (
	gwire "github.com/google/wire"
	y "l7e.io/yama/v2"
)

func NewApp(opts ...y.Option) (*App, y.Lifecycle, error) {
	panic(gwire.Build(NewA))
}
`)

	stub := info.Stubs()[0]

	assert.True(t, stub.HasOpts)
	assert.Empty(t, stub.GraphParams(), "the options parameter leaves the graph")
	assert.Equal(t, "y", stub.YamaName())
	assert.Equal(t, "gwire", stub.WireName(), "the stub's own alias for Google Wire")
}

func TestCollectPackageInfoReportsWhatReadingTheStubFilesProduced(t *testing.T) {
	_, err := pkg.CollectPackageInfo(filepath.Join(stubRoot, "unparseable"), nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "parse lifecycle.go")
}

// A directory that declares no stub states no import, so no name has to
// resolve. The path that the package declares still reaches the caller.
func TestCollectPackageInfoTakesADirectoryThatDeclaresNoStub(t *testing.T) {
	info, err := pkg.CollectPackageInfo("../wire", nil)

	require.NoError(t, err)
	assert.Empty(t, info.Stubs())
	assert.Equal(t, "l7e.io/yama/v2/internal/generator/wire", info.PkgPath())
}
