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

package wirerun

import (
	"context"

	"gopkg.in/yaml.v3"
)

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

// DB carries a cleanup and declares no capability.
type DB struct{}

func NewDB(l *Logger) (*DB, func()) { return &DB{}, func() {} }

// App declares two capabilities and carries no cleanup.
type App struct{}

func NewApp(db *DB, node *yaml.Node) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

// Boot calls the lifecycle constructor. Every load this package takes during a
// run must resolve that call.
func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx, nil)

	return err
}
