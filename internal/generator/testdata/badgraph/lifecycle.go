// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package badgraph

import (
	"context"

	"github.com/google/wire"

	"l7e.io/yama"
)

// NewAppLifecycle names no provider of *DB, so Google Wire rejects it.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewApp))
}
