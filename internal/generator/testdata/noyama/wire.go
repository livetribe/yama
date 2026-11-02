// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package noyama

import "github.com/google/wire"

// InitApp builds a graph whose only capable component is declared without any
// reference to yama.
func InitApp() *App {
	panic(wire.Build(NewWorker, wire.Struct(new(App), "*")))
}
