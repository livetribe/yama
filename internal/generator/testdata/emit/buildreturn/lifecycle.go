// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package buildreturn

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle calls wire.Build as a statement of its own and then returns.
func NewLifecycle() (*App, yama.Lifecycle, error) {
	wire.Build(NewApp)
	return nil, nil, nil
}
