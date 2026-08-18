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

import "maps"

// TakeOutputImportsFrom takes the import block of Google Wire's output onto the
// package. It also asks the Go toolchain for the name of each path that no stub
// file states.
//
// The construction that a lifecycle file reproduces can name a package that no
// stub file imports. A provider set states providers of its own, and Google
// Wire reaches them through its own import block.
//
// TakeOutputImportsFrom takes nothing from output that does not parse. It
// reports one name that two paths use. The lifecycle file carries both import
// blocks, and one name there refers to one path.
func (info *Info) TakeOutputImportsFrom(src []byte) error {
	block := ImportsIn(src)
	fresh := info.unresolvedPaths(block)
	resolved := Names(info.dir, fresh)

	return info.takeOutputImports(block, resolved)
}

// unresolvedPaths returns the path of each import in block that the package
// holds no name for. A run asks the Go toolchain for those paths alone.
func (info *Info) unresolvedPaths(block []Import) []string {
	var fresh []string

	for _, imp := range block {
		if _, known := info.importNames[imp.Path]; !known {
			fresh = append(fresh, imp.Path)
		}
	}

	return fresh
}

// takeOutputImports stores Google Wire's import block with the name that the
// toolchain resolved for each of its paths.
func (info *Info) takeOutputImports(block []Import, resolved map[string]string) error {
	info.outputImports = block

	if info.importNames == nil {
		info.importNames = make(map[string]string, len(resolved))
	}

	maps.Copy(info.importNames, resolved)

	all := info.AllImports()

	return CheckImports(all, info.importNames)
}
