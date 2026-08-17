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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

// shapesDir holds one package that declares a stub of each shape a person can
// write, over two stub files that state different aliases. Every fact that the
// tests over it assert comes out of that source.
const shapesDir = "testdata/shapes"

var (
	shapesOnce sync.Once
	shapesInfo *pkg.Info
	errShapes  error
)

// loadShapes reads the stub files of the shapes package. It states no fact that
// the Go toolchain holds.
func loadShapes(t *testing.T) *pkg.Info {
	t.Helper()

	info, err := pkg.Load(shapesDir, nil)
	require.NoError(t, err)

	return info
}

// collectShapes reads the shapes package and everything the Go toolchain states
// about it. It reads once for the whole run.
func collectShapes(t *testing.T) *pkg.Info {
	t.Helper()

	shapesOnce.Do(func() {
		shapesInfo, errShapes = pkg.CollectPackageInfo(shapesDir, nil)
	})

	require.NoError(t, errShapes)

	return shapesInfo
}

// shapeStub returns the stub of the given name that the shapes package
// declares.
func shapeStub(t *testing.T, name string) pkg.Stub {
	t.Helper()

	info := loadShapes(t)

	stubs := info.Stubs()
	for i := range stubs {
		if stubs[i].Name == name {
			return stubs[i]
		}
	}

	require.FailNowf(t, "no such stub", "the shapes package declares no %s", name)

	return pkg.Stub{}
}
