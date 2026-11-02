// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package dotwire

import (
	"context"

	. "github.com/google/wire"

	"l7e.io/yama"
	"l7e.io/yama/internal/generator/testdata/setlib"
)

// NewAppLifecycle imports Google Wire with a dot import.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(Build(setlib.ProviderSet, NewApp))
}
