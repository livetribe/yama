// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package diamond

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a diamond graph.
func NewLifecycle(opts ...yama.Option) (*Top, yama.Lifecycle, error) {
	panic(wire.Build(NewBase, NewLeft, NewRight, NewTop))
}
