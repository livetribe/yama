// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package capabilities

import "github.com/google/wire"

// InitApp builds a graph spanning the capability combinations: one component per
// single capability, one implementing all three, one implementing none, one whose
// method names match but whose signatures do not, one bound as an interface, one
// whose methods are on a receiver the bound value does not have, and a capable
// root.
func InitApp() *App {
	panic(wire.Build(
		NewOnlyStarter,
		NewOnlyQuiescer,
		NewOnlyStopper,
		NewDecoy,
		NewPlain,
		NewFull,
		NewGateway,
		NewValueStopper,
		wire.Struct(new(App), "*"),
	))
}
