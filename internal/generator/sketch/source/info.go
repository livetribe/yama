package source

import (
	"go/parser"
	"go/token"
	"maps"
	"strconv"

	"l7e.io/yama/v2/internal/generator/sketch/resolve"
)

// A PackageInfo is everything a run knows about one target package. It states
// what the package's stub files declare, and what the Go toolchain states about
// the package itself.
//
// A run collects one of these for each target package, and every later step
// takes from it the facts that step needs.
type PackageInfo struct {
	// Dir is the directory that holds the package.
	Dir string

	// Name is the package clause that the stub files carry.
	Name string

	// PkgPath is the import path that the package declares. It is empty for a
	// directory that the Go toolchain could not read.
	PkgPath string

	// Stubs are the lifecycle stubs that the package declares, in a stable
	// order: by file name, and then by position in the file.
	Stubs []StubInfo

	// Imports are the entries of every stub file's import block.
	Imports []Import

	// ImportNames holds the name that each imported package declares, keyed by
	// path. An import path does not always hold that name.
	ImportNames map[string]string

	// OutputImports are the entries of the import block that Google Wire wrote.
	// AddImportsFrom fills them, and they are empty until a run reads Google
	// Wire's output.
	OutputImports []Import
}

// AddImportsFrom takes the import block of Google Wire's output onto the
// package, and it resolves the name of each path that no stub file states.
//
// The construction that a lifecycle file reproduces may name a package that no
// stub file imports. A provider set states providers of its own, and Google
// Wire reaches them through its own import block.
//
// AddImportsFrom takes nothing from output that does not parse.
func (pi *PackageInfo) AddImportsFrom(src []byte) {
	found := importBlock(src)

	pi.OutputImports = found

	var fresh []string

	for _, imp := range found {
		if _, known := pi.ImportNames[imp.Path]; !known {
			fresh = append(fresh, imp.Path)
		}
	}

	resolved := resolve.Names(pi.Dir, fresh)
	maps.Copy(pi.ImportNames, resolved)
}

// importBlock returns the import block that src declares. It returns nothing
// for source that does not parse.
func importBlock(src []byte) []Import {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "imports.go", src, parser.ImportsOnly)
	if err != nil {
		return nil
	}

	imports := make([]Import, 0, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		imp := Import{Path: path}
		if spec.Name != nil {
			imp.Name = spec.Name.Name
		}

		imports = append(imports, imp)
	}

	return imports
}

// CollectPackageInfo reads everything a run knows about the package in path.
// tags are the build tags that the run set, and they reach both the stub files
// and the load that names the package.
//
// CollectPackageInfo reports the failure that reading the stub files produced.
// It reports none for a fact that the Go toolchain would not state: a package
// it cannot read leaves PkgPath empty, and a path it cannot read leaves that
// path out of ImportNames.
func CollectPackageInfo(path string, tags []string) (*PackageInfo, error) {
	info, err := Load(path, tags)
	if err != nil {
		return nil, err
	}

	info.PkgPath = resolve.PackagePath(path, withStubTag(tags))

	if len(info.Stubs) == 0 {
		return info, nil
	}

	paths := importPaths(info.Imports)
	info.ImportNames = resolve.Names(path, paths)

	if err := checkImports(info.Imports, info.ImportNames); err != nil {
		return nil, err
	}

	return info, nil
}

// importPaths returns the path of each import.
func importPaths(from []Import) []string {
	paths := make([]string, 0, len(from))

	for _, imp := range from {
		paths = append(paths, imp.Path)
	}

	return paths
}

// withStubTag puts the stub tag in front of the run's own tags. A package whose
// every file is a lifecycle stub holds no file without that tag.
func withStubTag(tags []string) []string {
	all := make([]string, 0, len(tags)+1)
	all = append(all, Tag)

	return append(all, tags...)
}
