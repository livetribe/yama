// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package capabilities

import (
	"context"

	"l7e.io/yama"
)

// One component per capability set, so the level list holds every shape a
// member can take, and one component that implements none of them.
var (
	_ yama.Starter  = (*StartOnly)(nil)
	_ yama.Quiescer = (*QuiesceOnly)(nil)
	_ yama.Stopper  = (*StopOnly)(nil)

	_ yama.Starter  = (*Everything)(nil)
	_ yama.Quiescer = (*Everything)(nil)
	_ yama.Stopper  = (*Everything)(nil)
)

type Inert struct{}

type StartOnly struct{ inert *Inert }

type QuiesceOnly struct{ inert *Inert }

type StopOnly struct{ inert *Inert }

type Everything struct {
	start   *StartOnly
	quiesce *QuiesceOnly
	stop    *StopOnly
}

func NewInert() *Inert { return &Inert{} }

func NewStartOnly(inert *Inert) *StartOnly { return &StartOnly{inert: inert} }

func NewQuiesceOnly(inert *Inert) *QuiesceOnly { return &QuiesceOnly{inert: inert} }

func NewStopOnly(inert *Inert) *StopOnly { return &StopOnly{inert: inert} }

func NewEverything(start *StartOnly, quiesce *QuiesceOnly, stop *StopOnly) *Everything {
	return &Everything{start: start, quiesce: quiesce, stop: stop}
}

func (*StartOnly) Start(context.Context) error { return nil }

func (*QuiesceOnly) Quiesce(context.Context) {}

func (*StopOnly) Stop(context.Context) {}

func (*Everything) Start(context.Context) error { return nil }

func (*Everything) Quiesce(context.Context) {}

func (*Everything) Stop(context.Context) {}
