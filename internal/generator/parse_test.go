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
	"errors"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compSummary is a comparable projection of a Component for table assertions.
type compSummary struct {
	Name     string
	Provider string
	Deps     []string
	Cleanup  string
}

func depNames(c *Component) []string {
	var names []string
	for _, d := range c.Deps {
		names = append(names, d.Name)
	}

	return names
}

func summarize(inj *Injector) []compSummary {
	out := make([]compSummary, 0, len(inj.Components))
	for _, c := range inj.Components {
		var deps []string
		for _, d := range c.Deps {
			deps = append(deps, d.Name)
		}

		cleanup := ""
		if c.Cleanup != nil {
			cleanup = c.Cleanup.Name
		}

		out = append(out, compSummary{Name: c.Name, Provider: c.Provider.String(), Deps: deps, Cleanup: cleanup})
	}

	return out
}

func parseFile(t *testing.T, path string) *ParsedFile {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	require.NoError(t, err)

	parsed, err := Parse(fset, file)
	require.NoError(t, err)

	return parsed
}

func injectorByName(t *testing.T, parsed *ParsedFile, name string) *Injector {
	t.Helper()

	for _, inj := range parsed.Injectors {
		if inj.Name == name {
			return inj
		}
	}

	t.Fatalf("injector %s not found", name)

	return nil
}

// TestParseSandboxOracle asserts the full extracted model for both injectors of
// the sandbox's real wire_gen.go, statement by statement: provider kinds, ordered
// dependency edges, cleanup pairing, and the root.
func TestParseSandboxOracle(t *testing.T) {
	parsed := parseFile(t, "sandbox/wire_gen.go")

	require.Len(t, parsed.Injectors, 2, "two injectors, parsed independently")

	app := injectorByName(t, parsed, "InitializeApp")
	assert.Empty(t, app.Params)
	assert.Equal(t, []compSummary{
		{Name: "writer", Provider: "value"},
		{Name: "config", Provider: "value"},
		{Name: "consoleLogger", Provider: "call", Deps: []string{"writer", "config"}},
		{Name: "base1", Provider: "call", Deps: []string{"consoleLogger"}},
		{Name: "base2", Provider: "call", Deps: []string{"consoleLogger"}, Cleanup: "cleanup"},
		{Name: "mid1", Provider: "call", Deps: []string{"consoleLogger", "base1", "base2"}},
		{Name: "root1", Provider: "call", Deps: []string{"consoleLogger", "mid1"}},
		{Name: "root2", Provider: "call", Deps: []string{"consoleLogger", "mid1"}},
		{Name: "environment", Provider: "fieldsOf", Deps: []string{"config"}},
		{Name: "base3", Provider: "call", Deps: []string{"consoleLogger", "environment"}, Cleanup: "cleanup2"},
		{Name: "mid2", Provider: "call", Deps: []string{"consoleLogger", "base2", "base3"}},
		{Name: "root3", Provider: "call", Deps: []string{"consoleLogger", "mid2"}},
		{Name: "app", Provider: "struct", Deps: []string{"root1", "root2", "root3"}},
	}, summarize(app))

	assert.Equal(t, "app", app.ResultName())
	appType := app.ResultType()
	assert.Equal(t, "*App", types.ExprString(appType))

	withWriter := injectorByName(t, parsed, "InitializeAppWithWriter")
	require.Len(t, withWriter.Params, 1)
	assert.Equal(t, "w", withWriter.Params[0].Name)
	assert.Equal(t, []compSummary{
		{Name: "config", Provider: "value"},
		{Name: "consoleLogger", Provider: "call", Deps: []string{"config"}},
		{Name: "base1", Provider: "call", Deps: []string{"consoleLogger"}},
		{Name: "base2", Provider: "call", Deps: []string{"consoleLogger"}, Cleanup: "cleanup"},
		{Name: "mid1", Provider: "call", Deps: []string{"consoleLogger", "base1", "base2"}},
		{Name: "root1", Provider: "call", Deps: []string{"consoleLogger", "mid1"}},
		{Name: "root2", Provider: "call", Deps: []string{"consoleLogger", "mid1"}},
		{Name: "environment", Provider: "fieldsOf", Deps: []string{"config"}},
		{Name: "base3", Provider: "call", Deps: []string{"consoleLogger", "environment"}, Cleanup: "cleanup2"},
		{Name: "mid2", Provider: "call", Deps: []string{"consoleLogger", "base2", "base3"}},
		{Name: "root3", Provider: "call", Deps: []string{"consoleLogger", "mid2"}},
		{Name: "app", Provider: "struct", Deps: []string{"root1", "root2", "root3"}},
	}, summarize(withWriter))
}

// TestParseIndependentGraphs asserts the two injectors are never merged: they
// hold distinct component instances even for identically named locals.
func TestParseIndependentGraphs(t *testing.T) {
	parsed := parseFile(t, "sandbox/wire_gen.go")

	app := injectorByName(t, parsed, "InitializeApp")
	withWriter := injectorByName(t, parsed, "InitializeAppWithWriter")

	appByName := map[string]*Component{}
	for _, c := range app.Components {
		appByName[c.Name] = c
	}

	for _, c := range withWriter.Components {
		if other, ok := appByName[c.Name]; ok {
			assert.NotSame(t, other, c, "component %s must not be shared across injectors", c.Name)
		}
	}
}

// TestParseValueRematerialization asserts a ProviderValue component carries the
// initializer of its _wire*Value variable, for re-emission in place of the
// private reference.
func TestParseValueRematerialization(t *testing.T) {
	parsed := parseFile(t, "sandbox/wire_gen.go")
	app := injectorByName(t, parsed, "InitializeApp")

	byName := map[string]*Component{}
	for _, c := range app.Components {
		byName[c.Name] = c
	}

	writer := byName["writer"]
	require.NotNil(t, writer.ValueExpr)
	assert.Equal(t, "os.Stdout", types.ExprString(writer.ValueExpr))

	config := byName["config"]
	require.NotNil(t, config.ValueExpr)
	assert.Contains(t, types.ExprString(config.ValueExpr), "Config{")
}

// TestParseStructComponent asserts a wire.Struct feeding a downstream provider is
// a component, and that a returned value produced by a provider call rather than
// a struct literal is both the injector's root and an ordinary component.
func TestParseStructComponent(t *testing.T) {
	parsed := parseFile(t, "testdata/structcomp/wire_gen.go")
	require.Len(t, parsed.Injectors, 1)

	inj := parsed.Injectors[0]
	assert.Equal(t, []compSummary{
		{Name: "a", Provider: "call"},
		{Name: "b", Provider: "call", Cleanup: "cleanup"},
		{Name: "cfg", Provider: "struct", Deps: []string{"a", "b"}},
		{Name: "server", Provider: "call", Deps: []string{"cfg"}},
		{Name: "root", Provider: "call", Deps: []string{"server"}},
	}, summarize(inj))

	assert.Equal(t, "root", inj.ResultName())
	rootType := inj.ResultType()
	assert.Equal(t, "*Root", types.ExprString(rootType))

	// The cleanup's position is recorded for locating it.
	var b *Component
	for _, c := range inj.Components {
		if c.Name == "b" {
			b = c
		}
	}
	require.NotNil(t, b)
	require.NotNil(t, b.Cleanup)
	assert.True(t, b.Cleanup.Pos.IsValid(), "cleanup position is recorded")
}

// TestParseResultCleanup asserts a cleanup returned by the provider of the value
// the injector returns pairs with that value's component, like any other cleanup.
func TestParseResultCleanup(t *testing.T) {
	parsed := parseFile(t, "testdata/resultcleanup/wire_gen.go")
	require.Len(t, parsed.Injectors, 1)

	inj := parsed.Injectors[0]
	assert.Equal(t, []compSummary{
		{Name: "dep", Provider: "call"},
		{Name: "app", Provider: "call", Deps: []string{"dep"}, Cleanup: "cleanup"},
	}, summarize(inj))

	assert.Equal(t, "app", inj.ResultName())
}

// TestParseMinimalInjector asserts an injector whose only creation event is the
// value it returns yields that one component, with no dependencies.
func TestParseMinimalInjector(t *testing.T) {
	parsed := parseFile(t, "testdata/minimal/wire_gen.go")
	require.Len(t, parsed.Injectors, 1)

	inj := parsed.Injectors[0]
	assert.Equal(t, []compSummary{
		{Name: "app", Provider: "struct"},
	}, summarize(inj))

	assert.Equal(t, "app", inj.ResultName())
	appType := inj.ResultType()
	assert.Equal(t, "*App", types.ExprString(appType))
}

// TestParseFieldNamesAreNotEdges guards that identifiers naming a struct field —
// a composite-literal key, or an intermediate field in a selector chain — never
// mint a dependency edge, even when they coincide with a component's name.
func TestParseFieldNamesAreNotEdges(t *testing.T) {
	parsed := parseFile(t, "testdata/fieldnames/wire_gen.go")
	require.Len(t, parsed.Injectors, 1)

	byName := map[string]*Component{}
	for _, c := range parsed.Injectors[0].Components {
		byName[c.Name] = c
	}

	// h := NewH(Opts{port: cfg}) — the key `port` matches a component but is not
	// consumed; only the value `cfg` is.
	h := byName["h"]
	require.NotNil(t, h)
	assert.Equal(t, []string{"cfg"}, depNames(h))

	// x := a.b.c — the intermediate field `b` matches a component but is not
	// consumed; only the base `a` is.
	x := byName["x"]
	require.NotNil(t, x)
	assert.Equal(t, []string{"a"}, depNames(x))

	// dup := NewDup(port, port) — one value consumed twice is one edge, not two.
	dup := byName["dup"]
	require.NotNil(t, dup)
	assert.Equal(t, []string{"port"}, depNames(dup))
}

// TestParseMalformedShapes asserts each unsupported injector shape fails with a
// locatable *ParseError — never a panic, never a silent omission — and that the
// three messages are meaningfully distinct rather than one canned string.
func TestParseMalformedShapes(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		substrings []string
		distinct   string
	}{
		{
			name:       "unrecognized-provider-form",
			path:       "testdata/malformed/binaryrhs/wire_gen.go",
			substrings: []string{"InitializeApp", "unrecognized provider form", "binary expression", "retries"},
			distinct:   "binary expression",
		},
		{
			name:       "no-clear-edges",
			path:       "testdata/malformed/fieldwrite/wire_gen.go",
			substrings: []string{"InitializeApp", "no traceable dependency edge", "field assignment", "base2"},
			distinct:   "field assignment",
		},
		{
			name:       "result-name-collision",
			path:       "testdata/malformed/resultcollision/wire_gen.go",
			substrings: []string{"InitializeInbound", "InitializeOutbound", "result type name collision", "App"},
			distinct:   "result type name collision",
		},
		{
			name:       "unpaired-cleanup",
			path:       "testdata/malformed/orphancleanup/wire_gen.go",
			substrings: []string{"InitApp", "cleanup function orphaned pairs with no component"},
			distinct:   "pairs with no component",
		},
	}

	messages := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tc.path, nil, parser.ParseComments)
			require.NoError(t, err)

			var parsed *ParsedFile
			require.NotPanics(t, func() {
				parsed, err = Parse(fset, file)
			})
			require.Error(t, err)
			assert.Nil(t, parsed)

			var pe *ParseError
			require.True(t, errors.As(err, &pe), "want *ParseError, got %T", err)
			assert.NotEmpty(t, pe.Injector, "error names the injector")
			assert.Positive(t, pe.Pos.Line, "error names a source position")

			for _, want := range tc.substrings {
				assert.Contains(t, err.Error(), want)
			}
			messages[tc.distinct] = err.Error()
		})
	}

	// Each distinguishing phrase appears in its own message and no other, proving
	// the diagnostics are shape-specific.
	for owner, msg := range messages {
		for other := range messages {
			if other == owner {
				continue
			}
			assert.NotContains(t, msg, other, "message for %q must not contain another shape's phrase", owner)
		}
	}
}
