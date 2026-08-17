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

// TakeStubFile takes what one stub file states onto the package: the package
// clause it carries, the stubs it declares, and the entries of its import
// block.
//
// Every stub file of one package carries the same package clause, so the last
// file to state one states what the package is called.
func (info *Info) TakeStubFile(name string, stubs []Stub, imports []Import) {
	info.name = name
	info.stubs = append(info.stubs, stubs...)
	info.imports = append(info.imports, imports...)
}

// TakePkgPath takes the import path that the Go toolchain states for the
// package. It is empty for a directory that the toolchain could not read.
func (info *Info) TakePkgPath(path string) {
	info.pkgPath = path
}

// TakeImportNames takes the name that the Go toolchain resolved for each path
// that the stub files import.
//
// TakeImportNames reports one name that two paths answer to. The derived
// injector file carries the imports of every stub file, and one name there
// answers to one path.
func (info *Info) TakeImportNames(resolved map[string]string) error {
	info.importNames = resolved

	return CheckImports(info.imports, info.importNames)
}

// ImportPaths returns the path of each import that the stub files state.
func (info *Info) ImportPaths() []string {
	paths := make([]string, 0, len(info.imports))

	for _, imp := range info.imports {
		paths = append(paths, imp.Path)
	}

	return paths
}
