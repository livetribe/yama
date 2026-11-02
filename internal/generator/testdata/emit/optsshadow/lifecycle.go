// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package optsshadow

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle names its options parameter rt, which is also the name the
// re-emitted body refers to a provider's package by. The body's import cannot
// move, so the parameter is what gives way.
func NewLifecycle(rt ...yama.Option) (*A, yama.Lifecycle, error) {
	panic(wire.Build(AppSet))
}
