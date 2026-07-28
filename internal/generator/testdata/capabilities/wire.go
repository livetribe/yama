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
