// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package nogen

import "github.com/google/wire"

// InitRoot builds Root. This package deliberately ships no committed wire_gen.go,
// so generation into it must leave no trace behind.
func InitRoot() *Root {
	panic(wire.Build(NewDep, NewRoot))
}
