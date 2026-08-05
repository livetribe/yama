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
	"fmt"
	"path/filepath"
	"strings"
)

// EmitAll runs the whole generation pipeline over patterns, from dir, and
// returns one lifecycle file per package that declares lifecycle stubs.
//
// It reads each package's stubs, writes the injectors it derives from them,
// runs Google Wire once over those packages, parses and analyzes what Wire
// produced, and renders the lifecycle file. It returns the files rather than
// writing them, so a caller decides what to do with a package whose committed
// file already matches.
//
// Both transient files are scoped before Wire runs and removed after, so a file
// of either name that Yama did not create is preserved and restored. The
// committed lifecycle file is scoped with them, and put back afterward.
//
// The derived file declares each constructor beside its injector. The committed
// lifecycle file declares the same constructors, and the two are never visible
// together. Scoping also keeps a stale committed copy out of both Wire's load
// and Yama's own. A package that declares no stub is skipped in silence, the way
// Wire skips a package with no injector.
func (g *Generator) EmitAll(ctx context.Context, dir string, patterns []string) (files []*LifecycleFile, err error) {
	stubPkgs, err := g.loadStubPackages(ctx, dir, patterns)
	if err != nil {
		return nil, err
	}
	if len(stubPkgs) == 0 {
		return nil, nil
	}

	dirs := make([]string, len(stubPkgs))
	for i, sp := range stubPkgs {
		dirs[i] = sp.Dir
	}

	scopes, err := openWireGenScopes(dirs, g.wireGenName, derivedFileName, lifecycleGenName)
	if err != nil {
		return nil, err
	}

	defer func() {
		// Joined rather than dropped when an earlier error exists: a restore failure
		// names the backup holding a caller's original, which is exactly the message
		// a caller must not lose behind a Wire diagnostic.
		if restoreErr := scopes.restore(); restoreErr != nil {
			files, err = nil, errors.Join(err, restoreErr)
		}
	}()

	for _, sp := range stubPkgs {
		if _, writeErr := writeDerivedInjectors(sp); writeErr != nil {
			return nil, writeErr
		}
	}

	wirePatterns, err := dirPatterns(dir, dirs)
	if err != nil {
		return nil, err
	}

	derived := derivedNames(stubPkgs)
	if err := g.runWire(ctx, dir, wirePatterns, derived); err != nil {
		return nil, err
	}

	var results []*LifecycleFile
	for _, sp := range stubPkgs {
		file, emitErr := g.emitPackage(ctx, sp)
		if emitErr != nil {
			return nil, emitErr
		}
		results = append(results, file)
	}

	return results, nil
}

// derivedNames are the injectors a run derives, over every stub package it
// covers. A diagnostic Google Wire reports against one of them is rewritten to
// name the stub instead.
func derivedNames(stubPkgs []*StubPackage) []string {
	var names []string
	for _, sp := range stubPkgs {
		for _, stub := range sp.Stubs {
			names = append(names, stub.DerivedName())
		}
	}

	return names
}

// loadStubPackages reads the lifecycle stubs of every package under patterns,
// skipping the packages that declare none.
func (g *Generator) loadStubPackages(ctx context.Context, dir string, patterns []string) ([]*StubPackage, error) {
	dirs, err := ResolvePackages(ctx, dir, patterns, g.stubBuildFlags)
	if err != nil {
		return nil, err
	}

	var stubPkgs []*StubPackage
	for _, d := range dirs {
		sp, err := g.LoadStubs(ctx, d)
		switch {
		case errors.Is(err, ErrNoStubs):
			continue
		case err != nil:
			return nil, err
		}
		stubPkgs = append(stubPkgs, sp)
	}

	return stubPkgs, nil
}

// emitPackage parses and analyzes what Wire produced for one stub package, then
// renders its lifecycle file.
func (g *Generator) emitPackage(ctx context.Context, sp *StubPackage) (*LifecycleFile, error) {
	derived := make([]string, len(sp.Stubs))
	for i, stub := range sp.Stubs {
		derived[i] = stub.DerivedName()
	}

	pkg, err := g.LoadInjectors(ctx, sp.Dir, derived)
	if err != nil {
		return nil, err
	}

	analysis, err := Analyze(pkg)
	if err != nil {
		return nil, err
	}

	return Emit(sp, pkg, analysis), nil
}

// dirPatterns turns the directories a run generates for into package patterns
// relative to wd.
//
// Wire is told the stub packages rather than the caller's own patterns, so a
// package that matched those patterns but declares no stub is left alone. An
// application's committed Wire output in such a package is never regenerated,
// and no scope is needed to protect it.
func dirPatterns(wd string, dirs []string) ([]string, error) {
	// The resolved directories are absolute, and the working directory is
	// whatever the caller passed, so both sides are made absolute before the
	// relative path between them is taken.
	base, err := filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("yama: resolving %s: %w", wd, err)
	}

	patterns := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		rel, err := filepath.Rel(base, dir)
		if err != nil {
			return nil, fmt.Errorf("yama: locating %s relative to %s: %w", dir, base, err)
		}

		slashed := filepath.ToSlash(rel)
		if slashed != "." && !strings.HasPrefix(slashed, "../") {
			slashed = "./" + slashed
		}
		patterns = append(patterns, slashed)
	}

	return patterns, nil
}
