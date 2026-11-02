// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
