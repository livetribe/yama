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
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// analyzedDirs memoizes each fixture directory's analysis. Running Google Wire
// and type-checking the result is what these tests spend their time on, and the
// analysis they share is only read.
var analyzedDirs sync.Map

// analyzeDir generates and analyzes the fixture package in dir, reusing the
// result across the tests that read it. A test that mutates what it is given, or
// that must observe an independent load, loads the package itself.
func analyzeDir(t *testing.T, dir string) *Analysis {
	t.Helper()
	requireGo(t)

	load := sync.OnceValues(func() (*Analysis, error) {
		pkg, err := NewGenerator(Options{}).Generate(context.Background(), dir)
		if err != nil {
			return nil, err
		}

		return Analyze(pkg)
	})

	stored, _ := analyzedDirs.LoadOrStore(dir, load)
	analyze := stored.(func() (*Analysis, error))

	analysis, err := analyze()
	require.NoError(t, err)

	return analysis
}

// sandboxAnalysis is the sandbox's analysis, loaded once. The sandbox is loaded
// rather than generated: its wire_gen.go is hand-massaged and must not be
// regenerated.
var sandboxAnalysis = sync.OnceValues(func() (*Analysis, error) {
	pkg, err := NewGenerator(Options{}).Load(context.Background(), "sandbox")
	if err != nil {
		return nil, err
	}

	return Analyze(pkg)
})

// analyzeSandbox returns the shared sandbox analysis.
func analyzeSandbox(t *testing.T) *Analysis {
	t.Helper()
	requireGo(t)

	analysis, err := sandboxAnalysis()
	require.NoError(t, err)

	return analysis
}

// analyzePackage analyzes an already-loaded package.
func analyzePackage(t *testing.T, pkg *LoadedPackage) *Analysis {
	t.Helper()

	analysis, err := Analyze(pkg)
	require.NoError(t, err)

	return analysis
}

// injectorAnalysis returns the named injector's analysis, failing the test when
// the package has none.
func injectorAnalysis(t *testing.T, analysis *Analysis, injector string) *InjectorAnalysis {
	t.Helper()

	ia := analysis.For(injector)
	require.NotNilf(t, ia, "no analysis for injector %s", injector)

	return ia
}

// capabilitiesByName projects an injector's levels to the capabilities detected
// for each component that occupies one.
func capabilitiesByName(ia *InjectorAnalysis) map[string]Capabilities {
	caps := map[string]Capabilities{}
	for _, level := range ia.Levels {
		for _, m := range level {
			caps[m.Component.Name] = m.Capabilities
		}
	}

	return caps
}

// assertOccupiesNoLevel asserts the injector created the named component and that
// the analysis placed it in no level, so the absence is a decision about a
// component that exists rather than one the parser never saw.
func assertOccupiesNoLevel(t *testing.T, ia *InjectorAnalysis, name string) {
	t.Helper()

	c := componentByName(t, ia.Injector, name)
	_, ok := ia.Member(c)
	assert.Falsef(t, ok, "%s must occupy no level", name)
}

// assertLevels asserts the analysis places exactly these components in exactly
// these levels. Membership within a level is order-independent: components in one
// level have no ordering constraint between them, and the order they appear in is
// Wire's statement order rather than anything Yama decides.
func assertLevels(t *testing.T, want [][]string, ia *InjectorAnalysis) {
	t.Helper()

	got := levelNames(ia.Levels)
	require.Lenf(t, got, len(want), "level count: got %v, want %v", got, want)
	for i := range want {
		assert.ElementsMatchf(t, want[i], got[i], "level %d: got %v, want %v", i, got[i], want[i])
	}
}

// TestAnalyzeSandboxLevels asserts the levels computed for the sandbox are the
// ones its hand-authored lifecycle_gen.go exemplar pins: base2 occupies a level
// for its cleanup alone, root2 is ordered after the base layer through mid1,
// which occupies none, and mid2 through its own base2 and base3 dependencies.
//
// Both injectors build the same graph, so both must produce the same levels.
func TestAnalyzeSandboxLevels(t *testing.T) {
	analysis := analyzeSandbox(t)
	require.Len(t, analysis.Injectors, 2)

	want := [][]string{
		{"base1", "base2", "base3"},
		{"mid2", "root2"},
		{"root3"},
	}
	for _, name := range []string{"InitializeApp", "InitializeAppWithWriter"} {
		ia := injectorAnalysis(t, analysis, name)
		assertLevels(t, want, ia)
	}
}

// TestAnalyzeSandboxCapabilities asserts detection agrees, component by
// component, with the compile-time interface assertions the sandbox declares.
func TestAnalyzeSandboxCapabilities(t *testing.T) {
	analysis := analyzeSandbox(t)
	ia := injectorAnalysis(t, analysis, "InitializeApp")

	want := map[string]Capabilities{
		"base1": CanStop,
		"base2": None,
		"base3": allCaps,
		"mid2":  allCaps,
		"root2": CanStart | CanStop,
		"root3": allCaps,
	}

	assert.Equal(t, want, capabilitiesByName(ia))
}

// TestAnalyzeCapabilityCombinations asserts every capability combination is
// detected: one component per single capability, one implementing all three, one
// implementing none, one whose method names match but whose signatures do not,
// and a capable root.
func TestAnalyzeCapabilityCombinations(t *testing.T) {
	dir := filepath.Join("testdata", "capabilities")

	analysis := analyzeDir(t, dir)
	ia := injectorAnalysis(t, analysis, "InitApp")

	assertLevels(t, [][]string{
		{"onlyStarter", "onlyQuiescer", "onlyStopper", "gateway"},
		{"full"},
		{"app"},
	}, ia)

	want := map[string]Capabilities{
		"onlyStarter":  CanStart,
		"onlyQuiescer": CanQuiesce,
		"onlyStopper":  CanStop,
		"gateway":      CanStart,
		"full":         allCaps,
		"app":          CanStop,
	}
	assert.Equal(t, want, capabilitiesByName(ia))

	assertOccupiesNoLevel(t, ia, "decoy")
	assertOccupiesNoLevel(t, ia, "plain")
}

// TestAnalyzeReadsTheBoundValuesType asserts capability detection answers for the
// type the injector variable actually holds, which is the type the generated code
// passes on:
//
//   - an interface-typed value contributes the capabilities its interface
//     declares, and not those only its concrete type has;
//   - a value whose capability method is declared on the pointer receiver
//     contributes nothing, since the bound value is not the pointer.
//
// Both are the answer Go itself gives for an assignment to the capability
// interface, so a component placed in a level is one the runtime's own type
// assertion will also find.
func TestAnalyzeReadsTheBoundValuesType(t *testing.T) {
	dir := filepath.Join("testdata", "capabilities")

	analysis := analyzeDir(t, dir)
	ia := injectorAnalysis(t, analysis, "InitApp")

	gateway := componentByName(t, ia.Injector, "gateway")
	m, ok := ia.Member(gateway)
	require.True(t, ok)
	assert.Equal(t, CanStart, m.Capabilities,
		"Gateway declares Start; Stop belongs to the concrete type alone")

	assertOccupiesNoLevel(t, ia, "valueStopper")
}

// TestAnalyzeWithoutYamaImport asserts a component is capable because it declares
// the method, not because its package refers to yama: the fixture imports only
// context, and its Worker still occupies a level as a Starter and Stopper.
func TestAnalyzeWithoutYamaImport(t *testing.T) {
	dir := filepath.Join("testdata", "noyama")

	analysis := analyzeDir(t, dir)
	ia := injectorAnalysis(t, analysis, "InitApp")

	assertLevels(t, [][]string{{"worker"}}, ia)
	assert.Equal(t, map[string]Capabilities{"worker": CanStart | CanStop}, capabilitiesByName(ia))
}

// TestAnalyzeCleanupOnlyComponent asserts a dependency-only component whose
// provider returned a cleanup occupies a level of its own, and that the
// components around it, having neither trait, occupy none.
func TestAnalyzeCleanupOnlyComponent(t *testing.T) {
	dir := filepath.Join("testdata", "structcomp")

	analysis := analyzeDir(t, dir)
	ia := injectorAnalysis(t, analysis, "InitRoot")

	assertLevels(t, [][]string{{"b"}}, ia)
	assert.Equal(t, None, ia.Levels[0][0].Capabilities)
	assert.NotNil(t, ia.Levels[0][0].Component.Cleanup)
}

// TestAnalyzeNoLifecycleComponents asserts a graph with nothing to run is not an
// error: it yields an empty level list.
func TestAnalyzeNoLifecycleComponents(t *testing.T) {
	dir := filepath.Join("testdata", "minimal")

	analysis := analyzeDir(t, dir)
	ia := injectorAnalysis(t, analysis, "InitApp")

	assert.Empty(t, ia.Levels)
}

// TestAnalyzeIsDeterministic asserts two independent loads of one package produce
// the identical level assignment. The loads are deliberately not shared:
// repeating the whole load is what makes the second assignment independent of
// the first.
func TestAnalyzeIsDeterministic(t *testing.T) {
	requireGo(t)

	ctx := context.Background()
	g := NewGenerator(Options{})

	var runs [][][]string
	for range 2 {
		pkg, err := g.Load(ctx, "sandbox")
		require.NoError(t, err)

		analysis := analyzePackage(t, pkg)
		ia := injectorAnalysis(t, analysis, "InitializeApp")
		names := levelNames(ia.Levels)
		runs = append(runs, names)
	}

	for _, got := range runs[1:] {
		assert.Equal(t, runs[0], got)
	}
}

// TestAnalyzeUnresolvedComponentType asserts a component whose type cannot be
// resolved is reported as a locatable error rather than silently treated as
// having no capability.
func TestAnalyzeUnresolvedComponentType(t *testing.T) {
	requireGo(t)

	pkg, err := NewGenerator(Options{}).Load(context.Background(), "sandbox")
	require.NoError(t, err)

	inj := pkg.Injectors[0]
	require.NotEmpty(t, inj.Components)

	delete(pkg.Package.TypesInfo.Defs, inj.Components[0].Ident)

	_, err = Analyze(pkg)
	require.Error(t, err)

	var analysisErr *AnalysisError
	require.ErrorAs(t, err, &analysisErr)
	assert.Equal(t, inj.Name, analysisErr.Injector)
	assert.Contains(t, analysisErr.Error(), "cannot resolve the type of")
}
