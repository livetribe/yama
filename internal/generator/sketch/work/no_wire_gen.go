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

type NoWireGen struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
}

var _ State = (*NoWireGen)(nil)

func (n *NoWireGen) PackagePath() (path string, ok bool) {
	panic("should never reach here")
}

func (n *NoWireGen) Prepare() State {
	panic("should never reach here")
}

func (n *NoWireGen) Generate() State {
	panic("should never reach here")
}

// Complete settles the package's files. Google Wire generated nothing here, so
// the package owes the run no error of its own.
func (n *NoWireGen) Complete() error {
	return settle(n.custodian, n.intermediates, nil)
}
