// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package tagged

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph NewDep and NewRoot build. NewDep is only
// visible under the "special" build tag.
func NewLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(NewDep, NewRoot))
}
