// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package badwire

import "github.com/google/wire"

// InitRoot asks wire to build Root, but no provider supplies *Base, so
// `wire gen` fails during its own analysis.
func InitRoot() (*Root, error) {
	panic(wire.Build(NewRoot))
}
