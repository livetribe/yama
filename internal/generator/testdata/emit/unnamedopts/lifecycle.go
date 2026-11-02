// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package unnamedopts

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle's stub binds no name to any parameter that it declares.
func NewLifecycle(*Config, ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
