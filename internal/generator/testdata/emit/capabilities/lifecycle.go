// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package capabilities

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates one component per capability set.
func NewLifecycle(opts ...yama.Option) (*Everything, yama.Lifecycle, error) {
	panic(wire.Build(NewInert, NewStartOnly, NewQuiesceOnly, NewStopOnly, NewEverything))
}
