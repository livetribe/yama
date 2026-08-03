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
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupInjectorSrc is Wire output whose providers return cleanups, including
// the unwinding Wire emits on the error path, where a cleanup is referred to
// somewhere other than the statement that bound it.
const cleanupInjectorSrc = `package p

func yama_New() (*Root, func(), error) {
	a, cleanup := NewA()
	b, cleanup2, err := NewB(a)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	root := NewRoot(a, b)
	return root, func() {
		cleanup2()
		cleanup()
	}, nil
}
`

// TestRebindCleanupsNamesEachCleanupForItsValue asserts a cleanup is bound under
// the name of the value it releases, at its binding and at every reference to it.
func TestRebindCleanupsNamesEachCleanupForItsValue(t *testing.T) {
	fset, inj := parseOneInjector(t, cleanupInjectorSrc)

	rebindCleanups(inj)

	body := printNode(fset, inj.FuncDecl.Body)
	assert.Contains(t, body, "a, aCleanup := NewA()")
	assert.Contains(t, body, "b, bCleanup, err := NewB(a)")
	assert.Contains(t, body, "aCleanup()", "the error path must call the rebound name")
	assert.NotContains(t, body, "cleanup2()", "no reference may keep Wire's positional name")
}

// TestRebindCleanupsIsRepeatable asserts rebinding an injector it already
// rebound leaves it unchanged. Rebinding edits what it reads, so a second pass
// over one loaded package must not derive a second round of names.
func TestRebindCleanupsIsRepeatable(t *testing.T) {
	fset, inj := parseOneInjector(t, cleanupInjectorSrc)

	rebindCleanups(inj)
	once := printNode(fset, inj.FuncDecl.Body)

	rebindCleanups(inj)
	twice := printNode(fset, inj.FuncDecl.Body)

	assert.Equal(t, once, twice)
}

// TestRebindCleanupsEscapesATakenName asserts the derived name gives way to a
// component that already carries it, rather than shadowing that component.
func TestRebindCleanupsEscapesATakenName(t *testing.T) {
	src := `package p

func yama_New() (*Root, func(), error) {
	a, cleanup := NewA()
	aCleanup := NewDecoy()
	root := NewRoot(a, aCleanup)
	return root, func() {
		cleanup()
	}, nil
}
`

	fset, inj := parseOneInjector(t, src)

	rebindCleanups(inj)

	body := printNode(fset, inj.FuncDecl.Body)
	assert.Contains(t, body, "a, aCleanup2 := NewA()")
	assert.Contains(t, body, "aCleanup := NewDecoy()")
}

// TestFreeNameEscapesWithTheLowestNumber asserts the escape rule: the wanted
// name when it is free, and otherwise that name with the lowest suffix from 2
// upward that no name in scope carries.
func TestFreeNameEscapesWithTheLowestNumber(t *testing.T) {
	taken := map[string]bool{}
	collides := func(name string) bool { return taken[name] }

	assert.Equal(t, "rt", freeName("rt", collides))
	assert.Equal(t, "yama", freeName("yama", collides))

	taken["rt"] = true
	assert.Equal(t, "rt2", freeName("rt", collides))

	taken["rt2"] = true
	assert.Equal(t, "rt3", freeName("rt", collides))
}

// TestFreeNameRecordsNothing asserts freeName leaves the scope alone. The scope
// is what a caller updates between two choices, so a name freeName recorded
// itself would make one choice depend on how the caller ordered the others.
func TestFreeNameRecordsNothing(t *testing.T) {
	taken := map[string]bool{}
	collides := func(name string) bool { return taken[name] }

	assert.Equal(t, "rt", freeName("rt", collides))
	assert.Equal(t, "rt", freeName("rt", collides))
}

// parseOneInjector parses a single-injector source and returns it with the
// FileSet its nodes belong to.
func parseOneInjector(t *testing.T, src string) (*token.FileSet, *Injector) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "wire_gen.go", src, parser.SkipObjectResolution)
	require.NoError(t, err)

	parsed, err := extractInjectors(fset, file, nil)
	require.NoError(t, err)
	require.Len(t, parsed.Injectors, 1)

	return fset, parsed.Injectors[0]
}
