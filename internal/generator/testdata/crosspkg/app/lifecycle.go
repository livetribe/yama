// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package app

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewAppLifecycle orchestrates the graph AppSet builds.
func NewAppLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(AppSet))
}
