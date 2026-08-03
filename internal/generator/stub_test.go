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
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadStubsReadsTheDeclaration asserts that a stub yields the constructor
// name, the parameters the derived injector takes, and the name the emitted
// constructor forwards its options under.
func TestLoadStubsReadsTheDeclaration(t *testing.T) {
	requireGo(t)

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), "testapp")
	require.NoError(t, err)
	require.Len(t, sp.Stubs, 1)

	stub := sp.Stubs[0]
	assert.Equal(t, "NewLifecycle", stub.Name)
	assert.Equal(t, "yama_NewLifecycle", stub.DerivedName())
	assert.Equal(t, "opts", stub.OptsName())

	var params []string
	for _, p := range stub.GraphParams() {
		params = append(params, p.Name)
	}
	assert.Equal(t, []string{"rec", "fault"}, params,
		"the derived injector takes the stub's parameters without the trailing options")
}

// TestLoadStubsAcceptsAStubWithoutOptions asserts a stub that declares no
// variadic Option parameter loads, and that every parameter it declares reaches
// the derived injector.
//
// Trimming a trailing options parameter that is not there would drop a real
// graph parameter, and Wire would then report a missing provider against a file
// the application never sees.
func TestLoadStubsAcceptsAStubWithoutOptions(t *testing.T) {
	requireGo(t)

	cases := map[string][]string{
		filepath.Join("testdata", "emit", "noopts"):   {"cfg"},
		filepath.Join("testdata", "stub", "noparams"): nil,
	}

	for dir, want := range cases {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)
			require.NoError(t, err)
			require.Len(t, sp.Stubs, 1)

			stub := sp.Stubs[0]
			assert.False(t, stub.HasOpts)

			var params []string
			for _, p := range stub.GraphParams() {
				params = append(params, p.Name)
			}
			assert.Equal(t, want, params)
		})
	}
}

// TestStubOptsName asserts the identifier the emitted constructor forwards its
// options under, for each form a stub can bind the parameter with.
//
// A name the constructor cannot read from is the failure that matters: the
// emitted file still parses, so gofmt accepts it and the run reports success,
// and the defect only appears when the application compiles the committed file.
func TestStubOptsName(t *testing.T) {
	cases := map[string]struct {
		declared string
		want     string
	}{
		"named":  {declared: "cfgOpts", want: "cfgOpts"},
		"absent": {declared: "", want: defaultOptsName},
		"blank":  {declared: blankIdent, want: defaultOptsName},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &Stub{Params: []Field{{Name: tc.declared}}, HasOpts: true}

			assert.Equal(t, tc.want, stub.OptsName())
		})
	}
}

// TestLoadStubsOrdersByFileThenPosition asserts stub order does not depend on
// the order the loader happened to return the package's files in, since that
// order decides the order the constructors are emitted in.
func TestLoadStubsOrdersByFileThenPosition(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "stub", "twofiles")

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)
	require.NoError(t, err)

	var names []string
	for _, stub := range sp.Stubs {
		names = append(names, stub.Name)
	}
	assert.Equal(t, []string{"NewSecondLifecycle", "NewFirstLifecycle"}, names)
}

// TestLoadStubsReportsNoStubs asserts that a package declaring no stub is a
// recognizable outcome a sweep can skip, not a generic failure.
func TestLoadStubsReportsNoStubs(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "minimal")

	_, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)

	assert.True(t, errors.Is(err, ErrNoStubs), "want ErrNoStubs, got %v", err)
}

// TestLoadStubsReportsAnUnparseableStubFile asserts a stub file Yama cannot read
// fails the load.
//
// Such a file yields no stubs, which is indistinguishable from a package that
// declares none unless the load errors are checked. Reporting it as ErrNoStubs
// would let a sweep skip the package in silence, leaving a stale lifecycle file
// in place and the run exiting clean.
func TestLoadStubsReportsAnUnparseableStubFile(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "stub", "unparseable")

	_, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNoStubs), "an unreadable package is not a package with nothing to generate")
	assert.Contains(t, err.Error(), "lifecycle.go", "the message locates the file that failed")
}

// TestLoadStubsResolvesImportNames asserts an import's name comes from the
// package it names, not from its path.
//
// A path whose last element is a version suffix does not name its package, and
// the derived injector file refers to that package by name, so guessing from
// the path drops the import and Wire then fails on an undefined identifier.
func TestLoadStubsResolvesImportNames(t *testing.T) {
	requireGo(t)

	dir := filepath.Join("testdata", "emit", "versioned")

	sp, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)
	require.NoError(t, err)

	const versioned = "l7e.io/yama/v2/internal/generator/testdata/emit/versioned/v2"
	assert.Equal(t, "thing", sp.ImportNames[versioned])
}

// TestLoadStubsRejectsAnUnusableSignature asserts that a stub Yama cannot derive
// an injector from fails generation with a located message, rather than being
// skipped and leaving the application without the constructor it declared.
func TestLoadStubsRejectsAnUnusableSignature(t *testing.T) {
	requireGo(t)

	cases := map[string]string{
		"badresults": "second result",
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "stub", name)

			_, err := NewGenerator(Options{}).LoadStubs(context.Background(), dir)

			var stubErr *StubError
			require.ErrorAs(t, err, &stubErr)
			assert.Equal(t, "NewLifecycle", stubErr.Stub)
			assert.Contains(t, stubErr.Msg, want)
			assert.NotZero(t, stubErr.Pos.Line, "the message must locate the stub")

			// The rendered message is what the command prints, so it carries
			// the stub's name and the position as well as the reason.
			assert.Contains(t, stubErr.Error(), "lifecycle stub NewLifecycle")
			assert.Contains(t, stubErr.Error(), "lifecycle.go:")
			assert.Contains(t, stubErr.Error(), want)
		})
	}
}
