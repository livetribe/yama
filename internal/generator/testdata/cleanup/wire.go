// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package cleanup

import "github.com/google/wire"

// InitApp builds a graph spanning the cleanup-folding cases: a cleanup on a value
// with no capability, a cleanup on a Stopper, a cleanup on a value whose only
// capability is not Stopper, a Stopper with no cleanup, a value carrying neither,
// and a cleanup on the root's own provider.
//
// Pool reaches Conn only through Plain, which occupies no level, so the level the
// folded cleanups land in depends on ordering transmitted through a component
// that receives no callback.
func InitApp() (*App, func()) {
	panic(wire.Build(
		NewPool,
		NewPlain,
		NewConn,
		NewCache,
		NewWorker,
		NewApp,
	))
}
