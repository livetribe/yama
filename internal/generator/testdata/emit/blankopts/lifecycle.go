// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package blankopts

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle binds the blank identifier to its options parameter, which the
// emitted constructor cannot forward under that name.
func NewLifecycle(_ ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
