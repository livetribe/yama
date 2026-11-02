// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package renamedgraphparam

// Config is the type of the graph parameter that the stub declares.
type Config struct{}

// App is the type of the value that the stub's graph builds.
type App struct{ config *Config }

func NewApp(c *Config) *App { return &App{config: c} }

// cfg has the name of the stub's graph parameter. The package block declares
// cfg.
var cfg = Config{}
