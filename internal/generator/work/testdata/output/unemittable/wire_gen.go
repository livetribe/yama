// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file parses and it does not type-check, so Generate fails at the step
// that reads what each component's type declares.

//go:build !wireinject

package app

import "context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	logger := NewMissing()
	app := NewApp(logger)
	return app, func() {}, nil
}
