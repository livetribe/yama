// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package buildreturn

import "context"

// App is the graph's one component.
type App struct{}

func NewApp() *App { return &App{} }

func (*App) Start(context.Context) error { return nil }

func (*App) Stop(context.Context) {}
