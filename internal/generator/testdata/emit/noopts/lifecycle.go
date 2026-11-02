// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package noopts

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph NewApp builds. It takes a graph parameter
// and no options.
func NewLifecycle(cfg *Config) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
