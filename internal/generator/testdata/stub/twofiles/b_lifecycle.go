// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package twofiles

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewFirstLifecycle is declared in the file that sorts second.
func NewFirstLifecycle(opts ...yama.Option) (*First, yama.Lifecycle, error) {
	panic(wire.Build(NewFirst))
}
