package resolve

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"
)

// Packages expands package patterns to the directory of each package they
// match. It takes Go's own pattern syntax, "./..." among it. No pattern means
// ".", which is what `wire gen` takes it to mean.
//
// Packages resolves from dir, and it sets tags on the load. Pass the tag that
// reveals a lifecycle stub beside the run's own tags: a build tag changes which
// packages a pattern matches, and a run must reach the set of directories that
// Google Wire reaches.
//
// Packages returns the directories sorted, one entry for each, and it reports
// every package that the load could not read.
func Packages(ctx context.Context, dir string, tags, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Mode:       packages.NeedName | packages.NeedFiles,
		BuildFlags: buildFlags(tags),
	}

	loaded, err := packages.Load(cfg, escape(patterns)...)
	if err != nil {
		return nil, fmt.Errorf("resolve %v: %w", patterns, err)
	}

	return directories(loaded)
}

// escape marks each pattern as a pattern. An argument that carries no such mark
// can read as a file list, and Google Wire marks its own patterns the same way.
func escape(patterns []string) []string {
	escaped := make([]string, len(patterns))

	for i, pattern := range patterns {
		escaped[i] = "pattern=" + pattern
	}

	return escaped
}

// buildFlags returns the flags that set tags on a load. It returns none when
// the caller passed no tag.
func buildFlags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	return []string{"-tags=" + join(tags)}
}

// join puts the tags together as one space-separated value.
func join(tags []string) string {
	joined := tags[0]

	for _, tag := range tags[1:] {
		joined += " " + tag
	}

	return joined
}

// directories returns the directory of each loaded package, without a repeat.
// A package that holds no Go file has no directory, and it contributes none.
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
