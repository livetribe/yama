// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package pkgscope

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph in a package that already declares rt.
func NewLifecycle(opts ...yama.Option) (*A, yama.Lifecycle, error) {
	panic(wire.Build(NewA))
}
