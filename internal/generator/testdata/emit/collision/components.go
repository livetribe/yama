// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package collision

import "context"

// Type names Google Wire turns into the locals rt and yama, which are the two
// names Yama wants for its own imports.
type Rt struct{}

type Yama struct{ rt *Rt }

type App struct{ yama *Yama }

func NewRt() *Rt { return &Rt{} }

func NewYama(rt *Rt) *Yama { return &Yama{rt: rt} }

func NewApp(yama *Yama) *App { return &App{yama: yama} }

func (*Rt) Start(context.Context) error { return nil }

func (*Yama) Start(context.Context) error { return nil }

func (*App) Start(context.Context) error { return nil }
