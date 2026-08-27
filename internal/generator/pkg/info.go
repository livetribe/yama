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

// These are the two packages that a lifecycle stub names.
const (
	// PackagePath is Yama's own package. A stub names its Option and its
	// Lifecycle through this import.
	PackagePath = "l7e.io/yama"

	// WirePackagePath is Google Wire's own package. A stub states its providers
	// in a call to this package's Build function.
	WirePackagePath = "github.com/google/wire"
)

// An Info is everything that a run knows about one target package. It states
// what the package's stub files declare, and what the Go toolchain states about
// the package itself.
//
// A run collects one Info for each target package. Every later step takes from
// that Info the facts that the step needs. An Info holds its own fields, so
// every write to a field goes through a method of its own. No caller can
// therefore store a name without the check that the name needs.
type Info struct {
	// dir is the directory that holds the package.
	dir string

	// name is the package clause that the stub files carry.
	name string

	// pkgPath is the import path that the package declares. It is empty for a
	// directory that the Go toolchain could not read.
	pkgPath string

	// stubs are the lifecycle stubs that the package declares, in a stable
	// order: by file name, and then by position in the file.
	stubs []Stub

	// imports are the entries of every stub file's import block.
	imports []Import

	// importNames holds the name that each imported package declares, keyed by
	// path. An import path does not always hold that name.
	importNames map[string]string

	// outputImports are the entries of the import block that Google Wire wrote.
	// They are empty until a run reads Google Wire's output.
	outputImports []Import
}

// NewInfo returns the facts of the package that dir holds, with nothing read
// yet. A reader fills the Info one stub file at a time.
func NewInfo(dir string) *Info {
	return &Info{dir: dir, importNames: make(map[string]string)}
}

// Dir returns the directory that holds the package.
func (info *Info) Dir() string {
	return info.dir
}

// Name returns the package clause that the stub files carry.
func (info *Info) Name() string {
	return info.name
}

// PkgPath returns the import path that the package declares. It is empty for a
// directory that the Go toolchain could not read.
func (info *Info) PkgPath() string {
	return info.pkgPath
}

// Stubs returns the lifecycle stubs that the package declares, in the order
// that a reader took them.
func (info *Info) Stubs() []Stub {
	return info.stubs
}

// Imports returns the entries of every stub file's import block.
func (info *Info) Imports() []Import {
	return info.imports
}

// NameOf returns the name that the package's files use for one import.
func (info *Info) NameOf(imp Import) string {
	return imp.NameIn(info.importNames)
}

// AllImports returns the stub files' own import block, followed by the block
// that Google Wire stated in its output. It holds only the stub files' own
// block until a run reads that output.
//
// The two blocks together are what one lifecycle file carries.
func (info *Info) AllImports() []Import {
	all := make([]Import, 0, len(info.imports)+len(info.outputImports))
	all = append(all, info.imports...)

	return append(all, info.outputImports...)
}

// ImportedAs returns the name that the package's stub files use for path. It is
// empty for a path that no stub file imports.
func (info *Info) ImportedAs(path string) string {
	for _, imp := range info.imports {
		if imp.Path == path {
			return info.NameOf(imp)
		}
	}

	return ""
}
