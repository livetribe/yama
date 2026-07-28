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

package capabilities

import (
	"context"

	yama "l7e.io/yama/v2"
)

// The var block asserts, at compile time, which lifecycle interfaces each
// component implements. Capability detection must reach the same answers the
// compiler does here.
var (
	_ yama.Starter  = (*OnlyStarter)(nil)
	_ yama.Quiescer = (*OnlyQuiescer)(nil)
	_ yama.Stopper  = (*OnlyStopper)(nil)

	_ yama.Starter  = (*Full)(nil)
	_ yama.Quiescer = (*Full)(nil)
	_ yama.Stopper  = (*Full)(nil)

	_ yama.Starter = (Gateway)(nil)
	_ yama.Stopper = (*ValueStopper)(nil)

	_ yama.Stopper = (*App)(nil)
)

// OnlyStarter implements Starter and nothing else.
type OnlyStarter struct{}

func NewOnlyStarter() *OnlyStarter {
	return &OnlyStarter{}
}

func (*OnlyStarter) Start(context.Context) error { return nil }

// OnlyQuiescer implements Quiescer and nothing else.
type OnlyQuiescer struct{}

func NewOnlyQuiescer() *OnlyQuiescer {
	return &OnlyQuiescer{}
}

func (*OnlyQuiescer) Quiesce(context.Context) {}

// OnlyStopper implements Stopper and nothing else.
type OnlyStopper struct{}

func NewOnlyStopper() *OnlyStopper {
	return &OnlyStopper{}
}

func (*OnlyStopper) Stop(context.Context) {}

// Decoy carries a method for each capability name, each with a signature that is
// a near miss: Start takes no context, Quiesce takes an int, and Stop returns an
// error. It implements none of the three interfaces.
type Decoy struct{}

func NewDecoy() *Decoy {
	return &Decoy{}
}

func (*Decoy) Start() error               { return nil }
func (*Decoy) Quiesce(int)                {}
func (*Decoy) Stop(context.Context) error { return nil }

// Plain implements no lifecycle interface and carries no cleanup, so it occupies
// no level. It sits between OnlyStarter and Full, which must stay ordered
// relative to each other through it.
type Plain struct{}

func NewPlain(*OnlyStarter, *Decoy) *Plain {
	return &Plain{}
}

// Full implements all three lifecycle interfaces.
type Full struct{}

func NewFull(*Plain, *OnlyQuiescer, *OnlyStopper) *Full {
	return &Full{}
}

func (*Full) Start(context.Context) error { return nil }
func (*Full) Quiesce(context.Context)     {}
func (*Full) Stop(context.Context)        {}

// Gateway is an interface that declares a capability method. Its provider returns
// the interface, so the injector binds a variable of interface type whose method
// set is the interface's own rather than the concrete value's.
type Gateway interface {
	Start(context.Context) error
	Serve()
}

// defaultGateway also implements Stopper, which Gateway does not declare, so only
// the Starter the interface declares is visible where the value is bound.
type defaultGateway struct{}

func NewGateway() Gateway {
	return &defaultGateway{}
}

func (*defaultGateway) Start(context.Context) error { return nil }
func (*defaultGateway) Serve()                      {}
func (*defaultGateway) Stop(context.Context)        {}

// ValueStopper declares Stop on its pointer receiver while its provider returns
// the value, so the injector binds a ValueStopper. *ValueStopper implements
// Stopper; ValueStopper does not.
type ValueStopper struct{}

func NewValueStopper() ValueStopper {
	return ValueStopper{}
}

func (*ValueStopper) Stop(context.Context) {}

// App is the graph's root, and implements Stopper: a capable root takes a level
// like any other component.
type App struct {
	full         *Full
	gateway      Gateway
	valueStopper ValueStopper
}

func (*App) Stop(context.Context) {}
