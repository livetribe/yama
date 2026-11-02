// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package single

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph of one component.
func NewLifecycle(opts ...yama.Option) (*Only, yama.Lifecycle, error) {
	panic(wire.Build(NewOnly))
}
