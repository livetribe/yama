// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package fanout

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a base with four independent leaves.
func NewLifecycle(opts ...yama.Option) (*Hub, yama.Lifecycle, error) {
	panic(wire.Build(
		NewBase,
		NewLeaf1,
		NewLeaf2,
		NewLeaf3,
		NewLeaf4,
		wire.Struct(new(Hub), "*"),
	))
}
