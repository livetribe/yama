// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file is what Google Wire writes for the stub in testdata/target. It
// parses and it type-checks, so Generate reaches the lifecycle file.

//go:build !wireinject

package app

import "context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	logger := NewLogger()
	db, cleanup := NewDB(logger)
	app := NewApp(db)
	return app, func() { cleanup() }, nil
}
