// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package wirefail

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle states NewApp and not NewDep, so Google Wire cannot build *App.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
