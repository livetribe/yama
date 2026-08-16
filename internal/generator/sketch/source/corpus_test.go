package source_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/source"
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
			_, err := source.Load(filepath.Join(stubRoot, name), nil)

			require.Error(t, err)
			assert.ErrorContains(t, err, words)
		})
	}
}

func TestLoadTakesAStubThatDeclaresNoParameter(t *testing.T) {
	pkg, err := source.Load(filepath.Join(stubRoot, "noparams"), nil)

	require.NoError(t, err)
	require.Len(t, pkg.Stubs, 1)
	assert.Empty(t, pkg.Stubs[0].Params)
}

// A package states its stubs one file at a time. The order that they reach the
// lifecycle file is the order of the file names, and then of the declarations
// in one file, so one run over one package writes what the run before it wrote.
func TestLoadOrdersStubsByFileName(t *testing.T) {
	pkg, err := source.Load(filepath.Join(stubRoot, "twofiles"), nil)

	require.NoError(t, err)
	require.Len(t, pkg.Stubs, 2)

	// a_lifecycle.go declares NewSecondLifecycle, and b_lifecycle.go declares
	// NewFirstLifecycle.
	assert.Equal(t, "NewSecondLifecycle", pkg.Stubs[0].Name)
	assert.Equal(t, "NewFirstLifecycle", pkg.Stubs[1].Name)
}
