package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
)

// The generator that this sketch replaces keeps a corpus of Google Wire output
// that no parser can derive an ordering from. Each fixture states one shape.
//
// These tests read that corpus. Every fixture in it came from outside the
// sketch, and each one states a shape that no fixture of the sketch's own
// covers.

// corpusRoot holds the corpus, and malformedRoot the part of it that states one
// unreadable shape per fixture.
const (
	corpusRoot    = "../../testdata"
	malformedRoot = corpusRoot + "/malformed"
)

// injectorNames are the injectors that the corpus declares. Parse takes the
// names it is asked for, and a fixture declares one or two of these.
var injectorNames = []string{"InitializeApp", "InitApp", "InitializeInbound", "InitializeOutbound"}

// rejected pairs each fixture with the words that its rejection carries.
var rejected = map[string]string{
	"binaryrhs":       "bound to a binary expression",
	"fieldwrite":      "through a field assignment",
	"orphancleanup":   "pairs with no component",
	"unresolvedvalue": "declares no value for",
}

func TestParseRejectsTheMalformedCorpus(t *testing.T) {
	for name, words := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := graph.Parse(malformedFixture(t, name), injectorNames)

			require.Error(t, err)
			assert.ErrorContains(t, err, words)
		})
	}
}

// The corpus also holds two injectors whose result types share an unqualified
// name and denote different types. The generator that this sketch replaces
// rejects that pair, because it derives a namespace from the unqualified name.
// A lifecycle file states the stub's own signature and derives no name from a
// result type, so the pair reaches no name of Yama's own.
func TestParseTakesTwoResultsThatShareAName(t *testing.T) {
	injectors, err := graph.Parse(malformedFixture(t, "resultcollision"), injectorNames)

	require.NoError(t, err)
	assert.Len(t, injectors, 2)
}

// malformedFixture returns the Google Wire output that one fixture holds.
func malformedFixture(t *testing.T, name string) []byte {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(malformedRoot, name, "wire_gen.go"))
	require.NoError(t, err)

	return src
}

// corpusFixture returns the Google Wire output that one fixture of the wider
// corpus holds.
func corpusFixture(t *testing.T, name string) []byte {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(corpusRoot, name, "wire_gen.go"))
	require.NoError(t, err)

	return src
}

// A field name is not a dependency. The corpus states the shape twice over: a
// key of a composite literal, and a field in the middle of a selection. Each
// one carries the name of another component, and neither one consumes it.
func TestParseTakesNoEdgeFromAFieldName(t *testing.T) {
	injectors, err := graph.Parse(corpusFixture(t, "fieldnames"), []string{"InitApp"})
	require.NoError(t, err)
	require.Len(t, injectors, 1)

	deps := map[string][]string{}
	for _, c := range injectors[0].Components {
		deps[c.Name] = c.Deps
	}

	// h := NewH(Opts{port: cfg}) takes the value, and not the key.
	assert.Equal(t, []string{"cfg"}, deps["h"])

	// x := a.b.c takes the base of the selection, and not the field between.
	assert.Equal(t, []string{"a"}, deps["x"])

	// dup := NewDup(port, port) takes one value twice, which is one edge.
	assert.Equal(t, []string{"port"}, deps["dup"])
}

// The cleanup that pairs with the value an injector returns is a cleanup like
// any other. Google Wire binds every cleanup to the one name "cleanup", and the
// lifecycle file holds each one under the name of the component it cleans up.
func TestParseTakesTheCleanupOfTheReturnedValue(t *testing.T) {
	injectors, err := graph.Parse(corpusFixture(t, "resultcleanup"), []string{"InitApp"})
	require.NoError(t, err)
	require.Len(t, injectors, 1)

	inj := injectors[0]

	assert.Equal(t, []graph.Component{
		{Name: "dep"},
		{Name: "app", Deps: []string{"dep"}, Cleanup: "appCleanup"},
	}, inj.Components)

	assert.Equal(t, "app", inj.Result)
	assert.Equal(t, []string{"dep := NewDep()", "app, appCleanup := NewApp(dep)"}, inj.Statements)
}

// An injector whose only creation is the value it returns yields that one
// component, and it consumes nothing.
func TestParseTakesAnInjectorOfOneComponent(t *testing.T) {
	injectors, err := graph.Parse(corpusFixture(t, "minimal"), []string{"InitApp"})
	require.NoError(t, err)
	require.Len(t, injectors, 1)

	assert.Equal(t, []graph.Component{{Name: "app"}}, injectors[0].Components)
	assert.Equal(t, "app", injectors[0].Result)
}
