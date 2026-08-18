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

package sketch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// The corpus of the generator that this sketch replaces holds packages that
// state one property of the analysis each: which capability a type declares,
// which level a component takes, and which builder call a member produces.
//
// Each of these packages holds Google Wire's own input and no lifecycle stub.
// These tests run Google Wire over a copy. They then put its output through the
// same three steps that a run uses.

// analyzed holds the levels of one injector, and the components that it left
// out.
type analyzed struct {
	levels  [][]graph.Member
	omitted map[string]bool
}

// names returns the name of each member of each level.
func (a analyzed) names() [][]string {
	out := make([][]string, len(a.levels))

	for i, level := range a.levels {
		for _, m := range level {
			out[i] = append(out[i], m.Name)
		}
	}

	return out
}

// byName returns the member that each component produced, keyed by the name of
// that component.
func (a analyzed) byName() map[string]graph.Member {
	members := map[string]graph.Member{}

	for _, level := range a.levels {
		for _, m := range level {
			members[m.Name] = m
		}
	}

	return members
}

// member returns the member that one component produced.
func (a analyzed) member(name string) (graph.Member, bool) {
	m, ok := a.byName()[name]

	return m, ok
}

// analyzeCorpus runs Google Wire over a copy of one corpus package. It returns
// the levels of the named injector.
func analyzeCorpus(t *testing.T, name, injector string) analyzed {
	t.Helper()
	requireGo(t)

	dir := copyFamily(t, name)
	ctx := context.Background()

	_, err := wire.Run(ctx, dir, []string{"."}, wire.Args{})
	require.NoError(t, err)

	src, err := os.ReadFile(filepath.Join(dir, wire.BaseOutputName))
	require.NoError(t, err)

	injectors, err := graph.Parse(src, []string{injector})
	require.NoError(t, err)
	require.Len(t, injectors, 1)

	filled, _, err := graph.Detect(dir, nil, injectors)
	require.NoError(t, err)
	require.Len(t, filled, 1)

	levels := graph.Levels(filled[0].Components)

	a := analyzed{levels: levels, omitted: map[string]bool{}}

	for _, c := range filled[0].Components {
		if _, ok := a.member(c.Name); !ok {
			a.omitted[c.Name] = true
		}
	}

	return a
}

// assertLevels asserts that the analysis puts exactly these components in
// exactly these levels. Two components in one level have no order between them.
// The membership of a level is therefore independent of the order.
func assertLevels(t *testing.T, want [][]string, a analyzed) {
	t.Helper()

	got := a.names()

	require.Lenf(t, got, len(want), "level count: got %v, want %v", got, want)

	for i := range want {
		assert.ElementsMatchf(t, want[i], got[i], "level %d: got %v, want %v", i, got[i], want[i])
	}
}

// A type declares a capability when it declares the method that the capability
// names. The corpus covers the combinations: one type for each single
// capability, one type that declares all three, one type with method names that
// match and signatures that do not, one type bound under an interface that
// declares one of the three, and one type with methods on a receiver that the
// bound value does not have.
func TestCorpusDetectsEveryCapabilityCombination(t *testing.T) {
	a := analyzeCorpus(t, "capabilities", "InitApp")

	assertLevels(t, [][]string{
		{"onlyStarter", "onlyQuiescer", "onlyStopper", "gateway"},
		{"full"},
		{"app"},
	}, a)

	want := map[string]graph.Capability{
		"onlyStarter":  graph.Start,
		"onlyQuiescer": graph.Quiesce,
		"onlyStopper":  graph.Stop,
		"gateway":      graph.Start,
		"full":         graph.Start | graph.Quiesce | graph.Stop,
		"app":          graph.Stop,
	}

	got := map[string]graph.Capability{}
	for name, m := range a.byName() {
		got[name] = m.Capabilities
	}

	assert.Equal(t, want, got)

	// decoy declares a near miss of each of the three. plain declares no
	// capability. valueStopper declares Stop on a receiver that the injector
	// never binds.
	for _, name := range []string{"decoy", "plain", "valueStopper"} {
		assert.Truef(t, a.omitted[name], "%s must take no level", name)
	}
}

// A component declares a capability when it declares the method. The package
// that declares it refers to Yama nowhere.
func TestCorpusDetectsACapabilityWithoutAnImportOfYama(t *testing.T) {
	a := analyzeCorpus(t, "noyama", "InitApp")

	assertLevels(t, [][]string{{"worker"}}, a)

	worker, ok := a.member("worker")
	require.True(t, ok)
	assert.Equal(t, graph.Start|graph.Stop, worker.Capabilities)
}

// A cleanup runs at the position of the value that it cleans up. The corpus
// states the cases in one graph: a cleanup on a value with no capability, a
// cleanup on a Stopper, a cleanup on a value with one capability that is not
// Stop, a Stopper with no cleanup, a value with neither, and a cleanup on the
// root's own provider.
func TestCorpusPutsACleanupAtThePositionOfItsValue(t *testing.T) {
	a := analyzeCorpus(t, "cleanup", "InitApp")

	assertLevels(t, [][]string{
		{"pool", "worker"},
		{"conn"},
		{"cache"},
		{"app"},
	}, a)

	// A member takes one of three builder calls. A cleanup with no capability
	// takes the cleanup call only. A cleanup with a capability takes both.
	want := map[string]graph.Member{
		"pool":   {Name: "pool", Capabilities: graph.None, Cleanup: "poolCleanup"},
		"worker": {Name: "worker", Capabilities: graph.Stop},
		"conn":   {Name: "conn", Capabilities: graph.Stop, Cleanup: "connCleanup"},
		"cache":  {Name: "cache", Capabilities: graph.Start, Cleanup: "cacheCleanup"},
		"app":    {Name: "app", Capabilities: graph.Stop, Cleanup: "appCleanup"},
	}

	assert.Equal(t, want, a.byName())
	assert.True(t, a.omitted["plain"], "plain must take no level")
}

// A component that Google Wire built out of a struct literal takes a level for
// the cleanup that its provider returned. It declares no capability of its own.
func TestCorpusTakesAComponentBuiltFromAStructLiteral(t *testing.T) {
	a := analyzeCorpus(t, "structcomp", "InitRoot")

	assertLevels(t, [][]string{{"b"}}, a)

	b, ok := a.member("b")
	require.True(t, ok)
	assert.Equal(t, graph.None, b.Capabilities)
	assert.NotEmpty(t, b.Cleanup)
}

// A graph with nothing to run is not a failure. It yields no level at all.
func TestCorpusTakesAGraphWithNothingToRun(t *testing.T) {
	a := analyzeCorpus(t, "minimal", "InitApp")

	assert.Empty(t, a.levels)
}

// A package that does not compile fails the run. The failure carries the
// compiler's own message. The message names the file and states what the
// compiler found there.
func TestCorpusReportsAPackageThatDoesNotCompile(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("..", "testdata", "typeerror")

	src, err := os.ReadFile(filepath.Join(dir, wire.BaseOutputName))
	require.NoError(t, err)

	injectors, err := graph.Parse(src, []string{"InitializeApp"})
	require.NoError(t, err)

	_, _, err = graph.Detect(dir, nil, injectors)

	require.Error(t, err)
	assert.ErrorContains(t, err, "components.go", "the message names the file that does not compile")
	assert.ErrorContains(t, err, "not an int", "the message carries what the compiler said")
}
