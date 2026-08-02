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

//go:build wireinject

package sandbox

import (
	"io"

	"github.com/google/wire"
)

// InitializeApp builds the whole graph, logging to os.Stdout. It returns the
// aggregated cleanup and error because Base2's provider contributes both.
func InitializeApp() (*App, func(), error) {
	panic(wire.Build(AppSet))
}

// InitializeAppWithWriter builds the same graph but takes the log destination as
// an argument, so the injector parameter feeds the graph in place of the
// os.Stdout interface value.
func InitializeAppWithWriter(w io.Writer) (*App, func(), error) {
	panic(wire.Build(CoreSet))
}
