// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package noparams

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle declares no parameters at all: no graph parameter, and no
// options parameter.
func NewLifecycle() (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
