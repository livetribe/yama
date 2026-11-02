// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package chain

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a three-component chain.
func NewLifecycle(opts ...yama.Option) (*C, yama.Lifecycle, error) {
	panic(wire.Build(NewA, NewB, NewC))
}
