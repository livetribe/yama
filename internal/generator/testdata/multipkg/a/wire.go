// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package a

import "github.com/google/wire"

// InitRoot builds a's graph.
func InitRoot() *Root {
	panic(wire.Build(NewDep, NewRoot))
}
