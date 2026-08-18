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

package pkg

import (
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ImportPath returns the import path that the package in dir declares. It sets
// tags on the load. A directory therefore still resolves if a build tag guards
// every file in it.
//
// ImportPath returns an empty path for a directory that holds no package. It
// reports no error for such a directory.
func ImportPath(dir string, tags []string) string {
	cfg := &packages.Config{
		Mode:       packages.NeedName,
		Dir:        dir,
		BuildFlags: BuildFlags(tags),
		Env:        append(os.Environ(), "GOPROXY=off", "GOWORK=off"),
	}

	loaded, err := packages.Load(cfg, ".")
	if err != nil || len(loaded) == 0 {
		return ""
	}

	return loaded[0].PkgPath
}

// Names returns the name that each path's package declares, keyed by path. It
// resolves each path from dir. The module of that directory states what each
// path means.
//
// Names leaves out a path that it could not read, and it reports no error for
// such a path. A caller that finds no entry for a path still has the path
// itself. The last element of a path names its package most of the time.
//
// Names reaches no network. It reads the module cache of the machine that it
// runs on. A path that a build of dir resolves is a path in that cache already.
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

	for _, loadedPkg := range loaded {
		if loadedPkg.Name == "" || loadedPkg.PkgPath == "" {
			continue
		}

		names[loadedPkg.PkgPath] = loadedPkg.Name
	}

	return names
}

// BuildFlags returns the flags that set tags on a load. It returns no flag when
// the caller passed no tag.
//
// Every load that a run makes sets the same tags. A load that set other tags
// would see another set of files. A provider that only one load can see builds
// no graph.
func BuildFlags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	return []string{"-tags=" + strings.Join(tags, " ")}
}
