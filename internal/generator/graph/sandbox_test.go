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

package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/internal/generator/graph"
)

// The corpus holds one sandbox. The graph of that package carries every shape
// that matters at one time, with a lifecycle file that a person wrote by hand.
// That file is the exemplar. The levels and the calls in it are what a
// generation has to produce.
//
// These tests read the sandbox. They run no tool and write no file, so the
// sandbox's own Google Wire output stays as a person massaged it.

// sandboxDir holds the sandbox.
const sandboxDir = "../testdata/sandbox"

// sandboxInjectors are the two injectors that the sandbox declares. Both build
// one graph, so both state the same levels.
var sandboxInjectors = []string{"InitializeApp", "InitializeAppWithWriter"}

// allCaps is every capability that a component can declare.
const allCaps = graph.Start | graph.Quiesce | graph.Stop

// analyzeSandbox parses the sandbox's Google Wire output and reads what each
// component's type declares.
func analyzeSandbox(t *testing.T) []graph.Injector {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(sandboxDir, "wire_gen.go"))
	require.NoError(t, err)

	injectors, err := graph.Parse(src, sandboxInjectors)
	require.NoError(t, err)
	require.Len(t, injectors, 2)

	filled, _, err := graph.Detect(sandboxDir, nil, injectors)
	require.NoError(t, err)

	return filled
}

// TestSandboxLevels asserts the levels that the exemplar pins. base2 occupies a
// level for its cleanup only. root2 comes after the base layer through mid1,
// and mid1 occupies no level. mid2 comes after its own two dependencies.
//
// The members of one level have no order between them. This test therefore
// asserts what each level holds, and not the order of those members. Levels
// states each level in the order that Google Wire built its members, and the
// exemplar states one level in another order. Both orders run the same
// lifecycle.
func TestSandboxLevels(t *testing.T) {
	want := [][]string{
		{"base1", "base2", "base3"},
		{"mid2", "root2"},
		{"root3"},
	}

	for _, inj := range analyzeSandbox(t) {
		t.Run(inj.Name, func(t *testing.T) {
			got := names(graph.Levels(inj.Components))

			require.Len(t, got, len(want))

			for i := range want {
				assert.ElementsMatchf(t, want[i], got[i], "level %d", i)
			}
		})
	}
}

// TestSandboxCapabilities asserts what each component's type declares, which
// the sandbox states for itself in its compile-time interface assertions.
func TestSandboxCapabilities(t *testing.T) {
	want := map[string]graph.Capability{
		"base1": graph.Stop,
		"base2": graph.None,
		"base3": allCaps,
		"mid2":  allCaps,
		"root2": graph.Start | graph.Stop,
		"root3": allCaps,
	}

	injectors := analyzeSandbox(t)
	got := make(map[string]graph.Capability, len(want))

	for _, level := range graph.Levels(injectors[0].Components) {
		for _, m := range level {
			got[m.Name] = m.Capabilities
		}
	}

	assert.Equal(t, want, got)
}

// TestSandboxCleanups asserts the call that each member takes, which the
// exemplar pins: base1 takes WithComponents, base2 takes WithCleanup, and base3
// takes WithCleanableComponent.
func TestSandboxCleanups(t *testing.T) {
	want := map[string]string{
		"base1": "WithComponents",
		"base2": "WithCleanup",
		"base3": "WithCleanableComponent",
		"mid2":  "WithComponents",
		"root2": "WithComponents",
		"root3": "WithComponents",
	}

	for _, inj := range analyzeSandbox(t) {
		t.Run(inj.Name, func(t *testing.T) {
			got := make(map[string]string, len(want))

			for _, level := range graph.Levels(inj.Components) {
				for _, m := range level {
					got[m.Name] = memberCall(m)
				}
			}

			assert.Equal(t, want, got)
		})
	}
}

// memberCall names the builder call that one member takes.
func memberCall(m graph.Member) string {
	if m.Cleanup == "" {
		return "WithComponents"
	}

	if m.Capabilities != graph.None {
		return "WithCleanableComponent"
	}

	return "WithCleanup"
}
