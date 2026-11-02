// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package noopts

import "context"

// Config is supplied by the caller, so it reaches the graph as a parameter
// rather than through a provider.
type Config struct{ Name string }

// App is the graph's one component.
type App struct{ cfg *Config }

func NewApp(cfg *Config) *App { return &App{cfg: cfg} }

func (*App) Start(context.Context) error { return nil }

func (*App) Stop(context.Context) {}
