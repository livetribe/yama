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

// CollectPackageInfo reads everything a run knows about the package in path.
// tags are the build tags that the run set, and they reach both the stub files
// and the load that names the package.
//
// CollectPackageInfo reports the failure that reading the stub files produced.
// It reports none for a fact that the Go toolchain would not state: a package
// it cannot read leaves PkgPath empty, and a path it cannot read leaves that
// path out of ImportNames.
func CollectPackageInfo(path string, tags []string) (*Info, error) {
	info, err := Load(path, tags)
	if err != nil {
		return nil, err
	}

	stubTags := withStubTag(tags)
	pkgPath := ImportPath(path, stubTags)

	info.TakePkgPath(pkgPath)

	if len(info.Stubs()) == 0 {
		return info, nil
	}

	paths := info.ImportPaths()
	resolved := Names(path, paths)

	if err := info.TakeImportNames(resolved); err != nil {
		return nil, err
	}

	return info, nil
}

// withStubTag puts the stub tag in front of the run's own tags. A package whose
// every file is a lifecycle stub holds no file without that tag.
func withStubTag(tags []string) []string {
	all := make([]string, 0, len(tags)+1)
	all = append(all, Tag)

	return append(all, tags...)
}
