// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package value

import "context"

// Banner is the value the stub supplies as a literal, so the emitted file must
// carry that literal instead of the variable Wire bound it to.
type Banner string

// App is the graph root, and it takes the value from a provider call.
type App struct{ banner Banner }

func NewApp(banner Banner) *App { return &App{banner: banner} }

func (*App) Start(context.Context) error { return nil }

func (*App) Stop(context.Context) {}
