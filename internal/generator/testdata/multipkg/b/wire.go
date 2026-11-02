// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package b

import "github.com/google/wire"

// InitRoot builds b's graph.
func InitRoot() *Root {
	panic(wire.Build(NewSvc, NewRoot))
}
