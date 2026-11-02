// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package valuerun

import (
	"context"

	"github.com/google/wire"

	"l7e.io/yama"
)

// NewAppLifecycle builds the app from a wire.Value provider.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(wire.Value(Config{Env: "prod"}), NewApp))
}
