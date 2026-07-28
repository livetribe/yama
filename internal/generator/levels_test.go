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

package generator

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allCaps is every capability at once. The cases below otherwise name the
// capability constants directly: what level computation reads is only whether a
// set is None.
var allCaps = CanStart | CanQuiesce | CanStop

// node declares one component of a test graph: the value it binds, the values it
// consumes, the capabilities its type implements, and whether its provider
// returned a cleanup.
type node struct {
	name    string
	deps    []string
	caps    Capabilities
	cleanup bool
}

// buildGraph assembles the parsed model a graph declaration describes. Nodes are
// listed in the statement order Wire would emit them, so a node's dependencies
// precede it.
func buildGraph(t *testing.T, nodes []node) (inj *Injector, caps map[*Component]Capabilities) {
	t.Helper()

	inj = &Injector{Name: "InitApp"}
	caps = map[*Component]Capabilities{}
	byName := map[string]*Component{}

	for _, n := range nodes {
		c := &Component{Name: n.name, Ident: ast.NewIdent(n.name), Provider: ProviderCall}
		if n.cleanup {
			c.Cleanup = &Cleanup{Name: n.name + "Cleanup"}
		}

		for _, dep := range n.deps {
			d, ok := byName[dep]
			require.Truef(t, ok, "%s depends on %s, which is not declared before it", n.name, dep)
			c.Deps = append(c.Deps, d)
		}

		inj.Components = append(inj.Components, c)
		byName[n.name] = c
		caps[c] = n.caps
	}

	return inj, caps
}

// levelNames projects a level list to the component names in each level, which is
// what the ordering assertions are about.
func levelNames(levels []Level) [][]string {
	out := make([][]string, 0, len(levels))
	for _, level := range levels {
		names := make([]string, 0, len(level))
		for _, m := range level {
			names = append(names, m.Component.Name)
		}
		out = append(out, names)
	}

	return out
}

// TestComputeLevels covers the graph shapes the level list must order correctly.
// Each case declares a graph and the exact level list it must produce; the same
// list is what startup walks forward and what quiesce and stop walk back.
func TestComputeLevels(t *testing.T) {
	cases := []struct {
		name  string
		nodes []node
		want  [][]string
	}{
		{
			// DB → {Router, Worker}. Read back, this is [router, worker] then [db].
			name: "diamond: independent dependents share a level",
			nodes: []node{
				{name: "db", caps: allCaps},
				{name: "router", deps: []string{"db"}, caps: allCaps},
				{name: "worker", deps: []string{"db"}, caps: allCaps},
			},
			want: [][]string{{"db"}, {"router", "worker"}},
		},
		{
			name: "chain: each dependent is in a strictly later level",
			nodes: []node{
				{name: "a", caps: CanStop},
				{name: "b", deps: []string{"a"}, caps: CanStop},
				{name: "c", deps: []string{"b"}, caps: CanStop},
			},
			want: [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name: "deep chain: one component per level",
			nodes: []node{
				{name: "c1", caps: CanStart},
				{name: "c2", deps: []string{"c1"}, caps: CanStart},
				{name: "c3", deps: []string{"c2"}, caps: CanStart},
				{name: "c4", deps: []string{"c3"}, caps: CanStart},
				{name: "c5", deps: []string{"c4"}, caps: CanStart},
				{name: "c6", deps: []string{"c5"}, caps: CanStart},
			},
			want: [][]string{{"c1"}, {"c2"}, {"c3"}, {"c4"}, {"c5"}, {"c6"}},
		},
		{
			name: "wide fan-out: unrelated components share one level",
			nodes: []node{
				{name: "l1", caps: allCaps},
				{name: "l2", caps: allCaps},
				{name: "l3", caps: allCaps},
				{name: "l4", caps: allCaps},
				{name: "l5", caps: allCaps},
				{name: "root", deps: []string{"l1", "l2", "l3", "l4", "l5"}, caps: allCaps},
			},
			want: [][]string{{"l1", "l2", "l3", "l4", "l5"}, {"root"}},
		},
		{
			// The headline edge case: b occupies no level, and a and c stay ordered
			// relative to each other across it.
			name: "transitive ordering through a non-capable component",
			nodes: []node{
				{name: "a", caps: allCaps},
				{name: "b", deps: []string{"a"}, caps: None},
				{name: "c", deps: []string{"b"}, caps: allCaps},
			},
			want: [][]string{{"a"}, {"c"}},
		},
		{
			name: "transitive ordering through a chain of non-capable components",
			nodes: []node{
				{name: "a", caps: CanQuiesce},
				{name: "b", deps: []string{"a"}, caps: None},
				{name: "c", deps: []string{"b"}, caps: None},
				{name: "d", deps: []string{"c"}, caps: None},
				{name: "e", deps: []string{"d"}, caps: CanQuiesce},
			},
			want: [][]string{{"a"}, {"e"}},
		},
		{
			// The level of a component is that of the deepest occupant it reaches,
			// not the nearest: c reaches a directly and b through the chain.
			name: "the deepest dependency decides the level",
			nodes: []node{
				{name: "a", caps: allCaps},
				{name: "b", deps: []string{"a"}, caps: allCaps},
				{name: "mid", deps: []string{"b"}, caps: None},
				{name: "c", deps: []string{"a", "mid"}, caps: allCaps},
			},
			want: [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name: "an interior component is ordered from both sides",
			nodes: []node{
				{name: "leaf", caps: allCaps},
				{name: "interior", deps: []string{"leaf"}, caps: allCaps},
				{name: "top", deps: []string{"interior"}, caps: allCaps},
			},
			want: [][]string{{"leaf"}, {"interior"}, {"top"}},
		},
		{
			name: "a capable root takes the last level",
			nodes: []node{
				{name: "dep", caps: allCaps},
				{name: "app", deps: []string{"dep"}, caps: CanStop},
			},
			want: [][]string{{"dep"}, {"app"}},
		},
		{
			name: "a dependency-only component with a cleanup occupies a level",
			nodes: []node{
				{name: "pool", caps: None, cleanup: true},
				{name: "server", deps: []string{"pool"}, caps: allCaps},
			},
			want: [][]string{{"pool"}, {"server"}},
		},
		{
			name: "a capable component with a cleanup occupies one level, not two",
			nodes: []node{
				{name: "pool", caps: allCaps, cleanup: true},
				{name: "server", deps: []string{"pool"}, caps: allCaps},
			},
			want: [][]string{{"pool"}, {"server"}},
		},
		{
			name: "a graph with nothing to run yields no levels",
			nodes: []node{
				{name: "config", caps: None},
				{name: "logger", deps: []string{"config"}, caps: None},
				{name: "app", deps: []string{"logger"}, caps: None},
			},
			want: [][]string{},
		},
		{
			name: "a single component",
			nodes: []node{
				{name: "app", caps: allCaps},
			},
			want: [][]string{{"app"}},
		},
		{
			name: "an injector with no components",
			want: [][]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inj, caps := buildGraph(t, tc.nodes)

			ia := computeLevels(inj, caps)

			assert.Equal(t, tc.want, levelNames(ia.Levels))
			for i, level := range ia.Levels {
				assert.NotEmptyf(t, level, "level %d is empty: levels must be contiguous", i)
			}
		})
	}
}

// TestComputeLevelsCarriesCapabilities asserts each member carries the
// capabilities detected for its component, which is what decides the passes it
// takes part in.
func TestComputeLevelsCarriesCapabilities(t *testing.T) {
	inj, caps := buildGraph(t, []node{
		{name: "pool", cleanup: true},
		{name: "server", deps: []string{"pool"}, caps: CanStart | CanStop},
	})

	ia := computeLevels(inj, caps)

	require.Len(t, ia.Levels, 2)
	require.Len(t, ia.Levels[0], 1)
	require.Len(t, ia.Levels[1], 1)
	assert.Equal(t, None, ia.Levels[0][0].Capabilities)
	assert.Equal(t, CanStart|CanStop, ia.Levels[1][0].Capabilities)
}

// TestCapabilitiesHas asserts a set contains exactly the capabilities that were
// put in it, and that every set contains None.
func TestCapabilitiesHas(t *testing.T) {
	cases := []struct {
		name  string
		caps  Capabilities
		has   []Capabilities
		lacks []Capabilities
	}{
		{
			name:  "none",
			caps:  None,
			has:   []Capabilities{None},
			lacks: []Capabilities{CanStart, CanQuiesce, CanStop},
		},
		{
			name:  "one capability",
			caps:  CanQuiesce,
			has:   []Capabilities{None, CanQuiesce},
			lacks: []Capabilities{CanStart, CanStop, CanQuiesce | CanStop},
		},
		{
			name:  "two capabilities",
			caps:  CanStart | CanStop,
			has:   []Capabilities{None, CanStart, CanStop, CanStart | CanStop},
			lacks: []Capabilities{CanQuiesce, allCaps},
		},
		{
			name: "all three",
			caps: allCaps,
			has:  []Capabilities{None, CanStart, CanQuiesce, CanStop, allCaps},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.has {
				assert.Truef(t, tc.caps.Has(want), "%q must have %q", tc.caps, want)
			}
			for _, unwanted := range tc.lacks {
				assert.Falsef(t, tc.caps.Has(unwanted), "%q must not have %q", tc.caps, unwanted)
			}
		})
	}
}

// TestCapabilitiesString asserts the rendering names capabilities in lifecycle
// order, whatever order they were combined in.
func TestCapabilitiesString(t *testing.T) {
	cases := []struct {
		caps Capabilities
		want string
	}{
		{caps: None, want: ""},
		{caps: CanStart, want: "start"},
		{caps: CanQuiesce, want: "quiesce"},
		{caps: CanStop, want: "stop"},
		{caps: CanStop | CanStart, want: "start stop"},
		{caps: allCaps, want: "start quiesce stop"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.caps.String())
		})
	}
}

// TestMemberKind asserts each combination of capabilities and cleanup produces
// the member the generated code must add for it. A cleanup pairs with the value
// whenever that value carries a capability of its own, whether or not the
// capability is Stop; it stands alone only when there is nothing to pair with.
func TestMemberKind(t *testing.T) {
	cases := []struct {
		name    string
		caps    Capabilities
		cleanup bool
		want    MemberKind
	}{
		{name: "a Stopper with no cleanup", caps: CanStop, want: MemberComponent},
		{name: "an all-capable value with no cleanup", caps: allCaps, want: MemberComponent},
		{name: "a cleanup on a value with no capability", cleanup: true, want: MemberCleanup},
		{name: "a cleanup on a Stopper", caps: CanStop, cleanup: true, want: MemberCleanableComponent},
		{name: "a cleanup on a Starter", caps: CanStart, cleanup: true, want: MemberCleanableComponent},
		{name: "a cleanup on a Quiescer", caps: CanQuiesce, cleanup: true, want: MemberCleanableComponent},
		{name: "a cleanup on an all-capable value", caps: allCaps, cleanup: true, want: MemberCleanableComponent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Component{Name: "c"}
			if tc.cleanup {
				c.Cleanup = &Cleanup{Name: "cCleanup"}
			}

			m := Member{Component: c, Capabilities: tc.caps}
			assert.Equal(t, tc.want, m.Kind())
		})
	}
}

// TestMemberKindPanicsOnTheZeroMember asserts Kind panics on the zero Member
// InjectorAnalysis.Member returns for a non-occupying component, rather than
// reading its nil Component as if it were a real answer.
func TestMemberKindPanicsOnTheZeroMember(t *testing.T) {
	assert.Panics(t, func() {
		Member{}.Kind()
	})
}

// TestMemberKindString asserts each kind renders as the name it is known by, and
// that an unnamed value renders as itself rather than as one of the three.
func TestMemberKindString(t *testing.T) {
	cases := []struct {
		kind MemberKind
		want string
	}{
		{kind: MemberComponent, want: "component"},
		{kind: MemberCleanableComponent, want: "cleanableComponent"},
		{kind: MemberCleanup, want: "cleanup"},
		{kind: MemberKind(7), want: "MemberKind(7)"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.kind.String())
		})
	}
}

// TestComputeLevelsIsDeterministic asserts repeated runs over one graph produce
// the identical level assignment, which is what makes emitted output stable.
func TestComputeLevelsIsDeterministic(t *testing.T) {
	inj, caps := buildGraph(t, []node{
		{name: "a", caps: allCaps},
		{name: "b", caps: allCaps},
		{name: "c", caps: None, cleanup: true},
		{name: "d", deps: []string{"a", "b"}, caps: None},
		{name: "e", deps: []string{"d", "c"}, caps: allCaps},
		{name: "f", deps: []string{"e"}, caps: CanStop},
		{name: "app", deps: []string{"f"}, caps: None},
	})

	ia := computeLevels(inj, caps)
	first := levelNames(ia.Levels)

	for range 32 {
		again := computeLevels(inj, caps)
		assert.Equal(t, first, levelNames(again.Levels))
	}
}

// componentByName finds a built graph's component by the value it binds.
func componentByName(t *testing.T, inj *Injector, name string) *Component {
	t.Helper()

	for _, c := range inj.Components {
		if c.Name == name {
			return c
		}
	}

	require.FailNowf(t, "no such component", "the graph has no component named %s", name)

	return nil
}

// TestInjectorLevelsMember asserts a component's placement is recoverable from
// the levels it produced, and that a component occupying no level is reported as
// absent rather than as a zero-valued member.
func TestInjectorLevelsMember(t *testing.T) {
	inj, caps := buildGraph(t, []node{
		{name: "pool", caps: None, cleanup: true},
		{name: "plain", deps: []string{"pool"}, caps: None},
		{name: "server", deps: []string{"plain"}, caps: CanStart | CanStop},
	})

	ia := computeLevels(inj, caps)

	server := componentByName(t, inj, "server")
	m, ok := ia.Member(server)
	require.True(t, ok)
	assert.Equal(t, server, m.Component)
	assert.Equal(t, CanStart|CanStop, m.Capabilities)

	pool := componentByName(t, inj, "pool")
	cleanupOnly, ok := ia.Member(pool)
	require.True(t, ok, "a cleanup-only component occupies a level")
	assert.Equal(t, None, cleanupOnly.Capabilities)

	plain := componentByName(t, inj, "plain")
	_, ok = ia.Member(plain)
	assert.False(t, ok, "a component occupying no level has no member")
}

// TestAnalysisFor asserts an injector's levels are found by name, and that an
// unknown name yields nil rather than an empty result that reads as an injector
// with no levels.
func TestAnalysisFor(t *testing.T) {
	analysis := &Analysis{Injectors: []*InjectorAnalysis{
		{Injector: &Injector{Name: "InitApp"}},
		{Injector: &Injector{Name: "InitWorker"}},
	}}

	worker := analysis.For("InitWorker")
	require.NotNil(t, worker)
	assert.Equal(t, "InitWorker", worker.Injector.Name)

	assert.Nil(t, analysis.For("InitNothing"))
}
