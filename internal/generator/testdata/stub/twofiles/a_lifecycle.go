// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package twofiles

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewSecondLifecycle is declared first in the file that sorts first, so stub
// order follows the file name and then the position rather than the name.
func NewSecondLifecycle(opts ...yama.Option) (*Second, yama.Lifecycle, error) {
	panic(wire.Build(NewSecond))
}
