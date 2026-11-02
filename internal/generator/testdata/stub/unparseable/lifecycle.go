// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package unparseable

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle is missing a closing paren, so this file does not parse.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp)
}
