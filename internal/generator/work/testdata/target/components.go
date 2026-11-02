// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// This file holds the components of the target package. NewDB returns a
// cleanup and declares no capability. App declares two.
//
// Boot calls the lifecycle constructor. The committed lifecycle file is set
// aside while Generate type-checks the package, so the package needs the
// placeholder that Prepare wrote to resolve that call.

package app

import "context"

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

type DB struct{}

func NewDB(l *Logger) (*DB, func()) { return &DB{}, func() {} }

type App struct{}

func NewApp(db *DB) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }
func (a *App) Stop(_ context.Context)        {}
