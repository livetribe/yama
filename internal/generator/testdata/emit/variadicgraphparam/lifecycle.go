// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package variadicgraphparam

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle declares a final parameter that is variadic but not
// yama.Option: a graph parameter, not an options parameter.
func NewLifecycle(names ...string) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
