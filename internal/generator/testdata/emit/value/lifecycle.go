// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package value

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph whose value provider holds a multi-line raw
// string. The golden records the literal with its own lines unchanged.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(wire.Value(Banner(`alpha
beta
gamma`)), NewApp))
}
