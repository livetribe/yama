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

package wire

import (
	"errors"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

// An IntermediateYamaFiles is the pair of files that a run puts in a target
// package for Google Wire's load: the derived injector file and the
// placeholder file. The two go in together, and they come out together.
type IntermediateYamaFiles struct {
	info *pkg.Info

	// written names each file that Prepare put in the directory. CleanUp takes
	// out these names and no other.
	written []string
}

// NewIntermediateYamaFiles returns the pair for one target package. info states
// the directory that holds the pair, and the stubs that each file declares.
func NewIntermediateYamaFiles(info *pkg.Info) *IntermediateYamaFiles {
	return &IntermediateYamaFiles{info: info}
}

// Prepare puts both files in the package's directory. It writes over neither
// name that the directory already holds, and it reports the first name that it
// found in place.
func (iyf *IntermediateYamaFiles) Prepare() error {
	derived := Derive(iyf.info)

	if err := iyf.write(DerivedFileName, derived); err != nil {
		return err
	}

	placeholder := DerivePlaceholder(iyf.info)

	return iyf.write(PlaceholderFileName, placeholder)
}

// CleanUp takes out each file that Prepare put in the package's directory, and
// no other. A file that is already gone is not an error.
func (iyf *IntermediateYamaFiles) CleanUp() error {
	errs := make([]error, 0, len(iyf.written))

	for _, name := range iyf.written {
		errs = append(errs, remove(iyf.info.Dir(), name))
	}

	iyf.written = nil

	return errors.Join(errs...)
}

// write puts one file in the package's directory and records its name.
func (iyf *IntermediateYamaFiles) write(name string, content []byte) error {
	if err := write(iyf.info.Dir(), name, content); err != nil {
		return err
	}

	iyf.written = append(iyf.written, name)

	return nil
}
