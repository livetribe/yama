// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package empty

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph with no level in it.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(wire.Struct(new(App), "*")))
}
