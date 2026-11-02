// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package optscollision

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewALifecycle names its options parameter after the runtime-support package,
// which the constructor's own body calls the builder on.
func NewALifecycle(rt ...yama.Option) (*A, yama.Lifecycle, error) {
	panic(wire.Build(NewA))
}

// NewBLifecycle names its options parameter after the public package, which the
// constructor's signature names.
func NewBLifecycle(yama ...yama.Option) (*B, yama.Lifecycle, error) {
	panic(wire.Build(NewB))
}
