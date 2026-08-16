// Package resolve reads the facts about a package that only the Go toolchain
// states: the directory that a pattern matches, and the name that an imported
// package declares.
//
// An import path does not always hold the name. A path that ends in a major
// version holds no name at all, and a path whose last element differs from the
// package clause holds the wrong one.
package resolve

import (
	"os"

	"golang.org/x/tools/go/packages"
)

// PackagePath returns the import path that the package in dir declares. It sets
// tags on the load, so a directory whose every file sits behind a build tag
// still resolves.
//
// PackagePath returns an empty path for a directory that holds no package, and
// it reports no error for one.
func PackagePath(dir string, tags []string) string {
	cfg := &packages.Config{
		Mode:       packages.NeedName,
		Dir:        dir,
		BuildFlags: buildFlags(tags),
		Env:        append(os.Environ(), "GOPROXY=off", "GOWORK=off"),
	}

	loaded, err := packages.Load(cfg, ".")
	if err != nil || len(loaded) == 0 {
		return ""
	}

	return loaded[0].PkgPath
}

// Names returns the name that each path's package declares, keyed by path. It
// resolves each path from dir, which is the directory whose module states what
// each path means.
//
// Names leaves out a path that it could not read, and it reports no error for
// one. A caller that finds no entry for a path still has the path itself to
// work from, and the last element of a path names its package most of the time.
//
// Names reaches no network. It reads the module cache of the machine it runs
// on. A path that a build of dir resolves is a path in that cache already.
func Names(dir string, paths []string) map[string]string {
	names := make(map[string]string, len(paths))

	if len(paths) == 0 {
		return names
	}

	cfg := &packages.Config{
		Mode: packages.NeedName,
		Dir:  dir,
		Env:  append(os.Environ(), "GOPROXY=off", "GOWORK=off"),
	}

	loaded, err := packages.Load(cfg, paths...)
	if err != nil {
		return names
	}

	for _, pkg := range loaded {
		if pkg.Name == "" || pkg.PkgPath == "" {
			continue
		}

		names[pkg.PkgPath] = pkg.Name
	}

	return names
}
