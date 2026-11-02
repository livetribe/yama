// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package wirefail

import "context"

// Dep is the value the stub's wire.Build call leaves without a provider.
type Dep struct{}

func NewDep() *Dep { return &Dep{} }

// App needs a Dep, so Google Wire rejects a graph that states no provider for
// one.
type App struct{ dep *Dep }

func NewApp(d *Dep) *App { return &App{dep: d} }

func (*App) Start(context.Context) error { return nil }
