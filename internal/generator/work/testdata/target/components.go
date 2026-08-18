// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
