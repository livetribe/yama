// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package setrun

import (
	"context"

	"github.com/google/wire"

	"l7e.io/yama"
	"l7e.io/yama/internal/generator/testdata/setlib"
)

// NewAppLifecycle names a provider set. It never names the package that
// declares the provider in that set.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(setlib.ProviderSet, NewApp))
}
