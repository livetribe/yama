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

// Lifecycle constructor stubs. Each stub fixes a constructor's name and
// signature and states the providers its graph is built from; yama derives a
// Wire injector from each one and emits the bodies into lifecycle_gen.go.

import (
	"io"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewLifecycle orchestrates the graph AppSet builds, logging to os.Stdout.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(AppSet))
}

// NewLifecycleWithWriter orchestrates the graph CoreSet builds, taking the log
// destination as an argument.
func NewLifecycleWithWriter(w io.Writer, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(CoreSet))
}
