// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package pkg

// TakeStubFile takes what one stub file states onto the package: the package
// clause that it carries, the stubs that it declares, and the entries of its
// import block.
//
// Every stub file of one package carries the same package clause, so the last
// file to state one states the name of the package.
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
// TakeImportNames reports one name that two paths use. The derived injector
// file carries the imports of every stub file, and one name there refers to one
// path.
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
