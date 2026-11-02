// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package structcomp

import "github.com/google/wire"

// InitRoot builds Root through a wire.Struct intermediate (Cfg) and a
// cleanup-returning provider (NewB).
func InitRoot() (*Root, func()) {
	panic(wire.Build(NewA, NewB, wire.Struct(new(Cfg), "*"), NewServer, NewRoot))
}
