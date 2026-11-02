// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package renamedgraphparam

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle's stub names its graph parameter cfg. The package block also
// declares cfg. Google Wire gives that parameter another name.
func NewLifecycle(cfg *Config) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
