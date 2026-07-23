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

package sandbox

// Lifecycle constructor stubs, the counterpart to wire.go. Each stub fixes a
// constructor's name and signature and names the Wire injector it is built from;
// yama emits the bodies into lifecycle_gen.go. Declaring them here is what makes
// lifecycle generation opt-in per injector, and what keeps constructor naming the
// application's choice rather than the generator's.

import (
	"io"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewLifecycle orchestrates the graph InitializeApp builds.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(InitializeApp))
}

// NewLifecycleWithWriter orchestrates the graph InitializeAppWithWriter builds,
// mirroring that injector's io.Writer argument.
func NewLifecycleWithWriter(w io.Writer, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(InitializeAppWithWriter))
}
