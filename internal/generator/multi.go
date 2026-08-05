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

// ResolvePackages expands package patterns to the directories of the packages
// they match, using Go's own package-pattern syntax (for example "./..."). No
// patterns defaults to ".", matching `wire gen`'s own default.
//
// buildFlags are applied to the package load. Pass Options.stubBuildFlags, so
// resolution loads under the tag that reveals a lifecycle stub and arrives at
// the package set a run generates for; a build tag can otherwise change which
// packages exist under a pattern.
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
