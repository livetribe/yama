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

package work

import (
	"io"

	"l7e.io/yama/internal/generator/custody"
	"l7e.io/yama/internal/generator/pkg"
	"l7e.io/yama/internal/generator/wire"
)

// CreateWorkItems returns one item for each target package. paths are the
// directories that hold them. prefix starts the name of every file that a run
// settles. header holds the bytes that every lifecycle file carries above
// Yama's own provenance line. header is nil when the run set no header file.
// tags are the build tags that the run set. progress receives each item's
// report of the file that it wrote.
//
// CreateWorkItems reads each package. The facts that it reads decide the first
// state of each package.
func CreateWorkItems(paths []string, prefix string, header []byte, tags []string, progress io.Writer) Items {
	items := make([]State, len(paths))

	for i, path := range paths {
		items[i] = createItem(path, prefix, header, tags, progress)
	}

	return items
}

// createItem returns the first state of the package in path. A package that
// the run cannot read starts as a CreateFailed. A package that declares no
// lifecycle stub starts as a NoStubs. Every other package starts as a Happy.
func createItem(path, prefix string, header []byte, tags []string, progress io.Writer) State {
	info, err := pkg.CollectPackageInfo(path, tags)
	if err != nil {
		return &CreateFailed{err: err}
	}

	if len(info.Stubs()) == 0 {
		return &NoStubs{}
	}

	custodian := custody.NewCustodian(path, prefix)
	intermediates := wire.NewIntermediateYamaFiles(info)

	return NewHappy(path, custodian, intermediates, header, tags, info, progress)
}
