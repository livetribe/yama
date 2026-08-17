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
	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// A NoWireGen is the item for a target package that holds no Google Wire
// output after Google Wire ran. It covers two directories that look the same:
// Google Wire rejected the first, and the second declared no stub for Google
// Wire to generate from. Complete puts the package's files back.
type NoWireGen struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
}

var _ State = (*NoWireGen)(nil)

// PackagePath reports no directory. Google Wire already ran, and the driver
// reads this call only to build the set that Google Wire runs over.
func (n *NoWireGen) PackagePath() (path string, runWire bool) {
	return "", false
}

// Prepare panics. The Prepare loop finished before this state existed.
func (n *NoWireGen) Prepare() State {
	panic("should never reach here")
}

// Generate panics. The driver calls Generate once for each item, and this
// state is what that one call returned.
func (n *NoWireGen) Generate() State {
	panic("should never reach here")
}

// Complete settles the package's files. Google Wire generated nothing here, so
// the package owes the run no error of its own.
func (n *NoWireGen) Complete() error {
	return settle(n.custodian, n.intermediates, nil)
}
