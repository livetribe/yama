// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package lib

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLibLifecycle orchestrates the graph LibSet builds.
func NewLibLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(LibSet))
}
