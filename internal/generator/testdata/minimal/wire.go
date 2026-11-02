// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package minimal

import "github.com/google/wire"

// InitApp builds an App with no dependency-bearing providers.
func InitApp() *App {
	panic(wire.Build(wire.Struct(new(App), "*")))
}
