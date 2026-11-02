// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package collision

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates a graph whose component locals take the names Yama
// wants for its own imports.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewRt, NewYama, NewApp))
}
