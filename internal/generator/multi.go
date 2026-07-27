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
	"sort"

	"golang.org/x/tools/go/packages"
)

// GenerateAll runs Google Wire once over patterns, from dir, and parses what it
// generated for each package.
//
// One invocation is how Wire runs itself, so its diagnostics, their ordering,
// and the resolution of a relative HeaderFile against dir are Wire's own rather
// than an approximation assembled from per-package runs.
//
// A failure on any package fails the whole run. Wire writes the packages that
// succeed before reporting the one that did not, but those files are transient
// here and are removed with the rest, so a failed run leaves the tree untouched
// rather than with some packages regenerated and others stale.
//
// Every directory Wire may write to is scoped before it runs and restored after,
// so Wire's output stays transient and a file Yama did not create is never lost.
// A package with no injectors is not a failure and is skipped silently, matching
// Wire's own silent no-op on such a package.
func (g *Generator) GenerateAll(ctx context.Context, dir string, patterns []string) (pkgs []*LoadedPackage, err error) {
	dirs, err := ResolvePackages(ctx, dir, patterns, g.discoveryBuildFlags)
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, nil
	}

	scopes, err := openWireGenScopes(dirs, g.wireGenName)
	if err != nil {
		return nil, err
	}

	defer func() {
		// Joined rather than dropped when an earlier error exists: a restore failure
		// names the backup holding a caller's original, which is exactly the message
		// a caller must not lose behind a Wire diagnostic.
		if restoreErr := scopes.restore(); restoreErr != nil {
			pkgs, err = nil, errors.Join(err, restoreErr)
		}
	}()

	// Wire failed, so the run failed. Wire writes the packages that succeed
	// before reporting the one that did not, but those files are transient here
	// and the deferred restore removes them: a failed run leaves the tree
	// untouched rather than half-regenerated.
	if err := g.runWire(ctx, dir, patterns); err != nil {
		return nil, err
	}

	var results []*LoadedPackage
	var errs []error
	for _, d := range dirs {
		pkg, loadErr := g.Load(ctx, d)
		switch {
		case errors.Is(loadErr, ErrNoInjectors):
			continue
		case loadErr != nil:
			errs = append(errs, loadErr)
		default:
			results = append(results, pkg)
		}
	}

	return results, errors.Join(errs...)
}

// ResolvePackages expands package patterns to the directories of the packages
// they match, using Go's own package-pattern syntax (for example "./..."). No
// patterns defaults to ".", matching `wire gen`'s own default.
//
// buildFlags are applied to the package load. Pass Options.discoveryBuildFlags,
// so resolution loads under the same tags Google Wire's loader uses and arrives
// at the same package set Wire will generate for; a build tag can otherwise
// change which packages exist under a pattern.
//
// Resolution runs before any Wire invocation. A transient-output scope must be
// open for a directory before Wire writes to it, which requires the concrete
// directory list up front rather than from Wire's own output afterward.
func ResolvePackages(ctx context.Context, dir string, patterns, buildFlags []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Mode:       packages.NeedName | packages.NeedFiles,
		BuildFlags: buildFlags,
	}

	// Patterns are escaped exactly as Google Wire escapes them, forcing pattern
	// interpretation rather than letting an argument be read as a file list. Wire
	// resolves the set of packages it writes to this way, and Yama must arrive at
	// the same set: a directory Yama fails to resolve is one it does not scope,
	// and so one Wire would write to unprotected.
	escaped := make([]string, len(patterns))
	for i, pattern := range patterns {
		escaped[i] = "pattern=" + pattern
	}

	pkgs, err := packages.Load(cfg, escaped...)
	if err != nil {
		return nil, fmt.Errorf("yama: resolving %v: %w", patterns, err)
	}

	var errs []error
	seen := map[string]bool{}
	var dirs []string
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			errs = append(errs, fmt.Errorf("yama: %s: %w", pkg.PkgPath, e))
		}
		if len(pkg.GoFiles) == 0 {
			continue
		}

		d := filepath.Dir(pkg.GoFiles[0])
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	sort.Strings(dirs)

	return dirs, nil
}
