// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package badresults

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle returns Google Wire's aggregated cleanup where a stub declares
// the Lifecycle.
func NewLifecycle(opts ...yama.Option) (*App, func(), error) {
	panic(wire.Build(NewApp))
}
