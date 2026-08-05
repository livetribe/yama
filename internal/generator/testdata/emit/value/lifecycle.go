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

//go:build yamainject

package value

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewLifecycle orchestrates a graph whose value provider holds a multi-line raw
// string. The golden records the literal with its own lines unchanged.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(wire.Value(Banner(`alpha
beta
gamma`)), NewApp))
}
