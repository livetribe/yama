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

package sandbox

import (
	"io"
	"os"

	"github.com/google/wire"
)

// sandboxLogPrefix tags every log line the sandbox app emits.
const sandboxLogPrefix = "[sandbox] "

// The component providers, grouped by graph layer.
var (
	BaseSet = wire.NewSet(NewBase1, NewBase2, NewBase3)
	MidSet  = wire.NewSet(NewMid1, NewMid2)
	RootSet = wire.NewSet(NewRoot1, NewRoot2, NewRoot3)
)

// InfraSet supplies the cross-cutting infrastructure the components depend on:
// the logger (interface bound to a concrete type), the configuration value, and
// the Environment projected out of it.
var InfraSet = wire.NewSet(
	NewConsoleLogger,
	wire.Bind(new(Logger), new(*ConsoleLogger)),
	wire.Value(Config{Env: Prod, LogPrefix: sandboxLogPrefix}),
	wire.FieldsOf(new(Config), "Env"),
)

// CoreSet wires the whole graph except the source of the log destination
// (io.Writer). Injectors compose it with a writer provider.
var CoreSet = wire.NewSet(
	InfraSet,
	BaseSet,
	MidSet,
	RootSet,
	wire.Struct(new(App), "*"),
)

// AppSet is CoreSet with os.Stdout bound as the io.Writer.
var AppSet = wire.NewSet(
	CoreSet,
	wire.InterfaceValue(new(io.Writer), os.Stdout),
)
