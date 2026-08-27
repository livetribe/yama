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

// Package resolve expands package patterns to the directories that they match.
//
// A run states its targets as patterns. Every later step works from a
// directory. pkg reads the facts of one directory.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"

	"l7e.io/yama/internal/generator/pkg"
)

// Packages expands package patterns to the directory of each package that they
// match. It uses Go's own pattern syntax. "./..." is one such pattern. No
// pattern means ".". `wire gen` gives the same meaning to no pattern.
//
// Packages resolves from dir, and it sets tags on the load. Pass the tag that
// reveals a lifecycle stub beside the run's own tags. A build tag changes which
// packages a pattern matches. A run must reach the set of directories that
// Google Wire reaches.
//
// Packages returns the directories sorted, one entry for each. It also reports
// every package that the load could not read.
func Packages(ctx context.Context, dir string, tags, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Mode:       packages.NeedName | packages.NeedFiles,
		BuildFlags: pkg.BuildFlags(tags),
	}

	loaded, err := packages.Load(cfg, escape(patterns)...)
	if err != nil {
		return nil, fmt.Errorf("resolve %v: %w", patterns, err)
	}

	return directories(loaded)
}

// escape marks each pattern as a pattern. Go can read an argument that carries
// no such mark as a file list. Google Wire marks its own patterns the same way.
func escape(patterns []string) []string {
	escaped := make([]string, len(patterns))

	for i, pattern := range patterns {
		escaped[i] = "pattern=" + pattern
	}

	return escaped
}

// directories returns the directory of each loaded package, without a repeat.
// A package that holds no Go file has no directory. It contributes no entry.
func directories(loaded []*packages.Package) ([]string, error) {
	var (
		errs []error
		dirs []string
	)

	seen := make(map[string]bool, len(loaded))

	for _, pkg := range loaded {
		for _, pkgErr := range pkg.Errors {
			errs = append(errs, fmt.Errorf("resolve %s: %w", pkg.PkgPath, pkgErr))
		}

		if len(pkg.GoFiles) == 0 {
			continue
		}

		dir := filepath.Dir(pkg.GoFiles[0])
		if seen[dir] {
			continue
		}

		seen[dir] = true
		dirs = append(dirs, dir)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	sort.Strings(dirs)

	return dirs, nil
}
