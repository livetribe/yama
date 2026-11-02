// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package b

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph NewRoot builds.
func NewLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(NewSvc, NewRoot))
}
