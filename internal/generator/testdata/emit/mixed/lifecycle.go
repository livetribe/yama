// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package mixed

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph in a package that also keeps Wire
// injectors of its own.
func NewLifecycle(opts ...yama.Option) (*A, yama.Lifecycle, error) {
	panic(wire.Build(NewA))
}
