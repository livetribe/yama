// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package cleanup

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph whose providers return cleanup functions.
func NewLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(NewPlain, NewCapable, NewFailing, NewRoot))
}
