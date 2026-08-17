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
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

// The generator that this sketch replaces keeps a corpus of stub packages. Each
// one states a stub that it takes or a stub that it rejects.
//
// These tests read that corpus. Every fixture in it came from outside the
// sketch.

// stubRoot holds the corpus.
const stubRoot = "../../testdata/stub"

func TestLoadRejectsTheStubCorpus(t *testing.T) {
	rejected := map[string]string{
		"badresults":      "declares func() as its second result",
		"optsnotvariadic": "declares yama.Option as its final parameter",
		"unparseable":     "parse lifecycle.go",
	}

	for name, words := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := pkg.Load(filepath.Join(stubRoot, name), nil)

			require.Error(t, err)
			assert.ErrorContains(t, err, words)
		})
	}
}

func TestLoadTakesAStubThatDeclaresNoParameter(t *testing.T) {
	info, err := pkg.Load(filepath.Join(stubRoot, "noparams"), nil)

	require.NoError(t, err)
	require.Len(t, info.Stubs(), 1)
	assert.Empty(t, info.Stubs()[0].Params)
}

// A package states its stubs one file at a time. The order that they reach the
// lifecycle file is the order of the file names, and then of the declarations
// in one file, so one run over one package writes what the run before it wrote.
func TestLoadOrdersStubsByFileName(t *testing.T) {
	info, err := pkg.Load(filepath.Join(stubRoot, "twofiles"), nil)

	require.NoError(t, err)
	require.Len(t, info.Stubs(), 2)

	// a_lifecycle.go declares NewSecondLifecycle, and b_lifecycle.go declares
	// NewFirstLifecycle.
	assert.Equal(t, "NewSecondLifecycle", info.Stubs()[0].Name)
	assert.Equal(t, "NewFirstLifecycle", info.Stubs()[1].Name)
}
