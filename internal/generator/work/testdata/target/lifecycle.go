// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file holds the lifecycle stub that a person wrote.

//go:build yamainject

package app

import (
	"context"

	"github.com/google/wire"

	"l7e.io/yama"
)

// NewAppLifecycle builds the app and its lifecycle.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewDB, NewApp))
}
