// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package dotlib

import (
	"context"

	"l7e.io/yama/internal/generator/testdata/setother"
)

type App struct{}

func NewApp(t *setother.Thing) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

