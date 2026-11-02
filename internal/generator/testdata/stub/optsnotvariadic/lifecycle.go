// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package optsnotvariadic

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle declares a final parameter of type yama.Option without the
// ellipsis that makes it the options parameter.
func NewLifecycle(opts yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
