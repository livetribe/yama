// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package bad

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle states no provider for Base, so Google Wire rejects it.
func NewLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(NewRoot))
}
