// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
