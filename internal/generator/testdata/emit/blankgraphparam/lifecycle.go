// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package blankgraphparam

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle's stub binds its graph parameter to the blank identifier.
// Google Wire gives that parameter a name.
func NewLifecycle(_ *Config) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
