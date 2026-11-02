// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file states a statement that no provider writes, so Generate fails at
// the step that reads the ordering out of the injector body.

//go:build !wireinject

package app

import "context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	go NewLogger()
	return nil, nil, nil
}
