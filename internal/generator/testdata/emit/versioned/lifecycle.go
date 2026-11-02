// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package versioned

import (
	"github.com/google/wire"

	"l7e.io/yama"
	"l7e.io/yama/internal/generator/testdata/emit/versioned/v2"
)

// NewLifecycle takes a parameter whose type comes from a package imported
// without an alias, under a path ending in a version suffix.
func NewLifecycle(c *thing.Client, opts ...yama.Option) (*A, yama.Lifecycle, error) {
	panic(wire.Build(NewA))
}
