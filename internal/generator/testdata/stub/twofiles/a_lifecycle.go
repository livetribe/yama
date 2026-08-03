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

package twofiles

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewSecondLifecycle is declared first in the file that sorts first, so stub
// order follows the file name and then the position rather than the name.
func NewSecondLifecycle(opts ...yama.Option) (*Second, yama.Lifecycle, error) {
	panic(wire.Build(NewSecond))
}
