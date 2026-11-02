// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package bad

import "github.com/google/wire"

// InitRoot asks wire to build Root, but no provider supplies *Base.
func InitRoot() (*Root, error) {
	panic(wire.Build(NewRoot))
}
