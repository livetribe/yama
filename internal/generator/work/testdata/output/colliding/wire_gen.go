// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file binds the name that the stub file binds to another path, so
// Generate fails at the step that reads the output's import block.

//go:build !wireinject

package app

import context "example.com/other/context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	app := NewApp(nil)
	return app, func() {}, nil
}
