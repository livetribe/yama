// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package mixed

import "github.com/google/wire"

// helper is an ordinary function the application keeps beside its injector.
// Google Wire copies it into wire_gen.go, because this file is tagged out of
// the normal build, and its body is nothing Yama's parser can read as an
// injector.
func helper(n int) int {
	for i := 0; i < n; i++ {
		n += i
	}

	return n
}

// AppSet is the application's own provider set.
var AppSet = wire.NewSet(NewA)

// InitializeA is the application's own injector, which Yama neither asked for
// nor reads.
func InitializeA() (*A, error) {
	panic(wire.Build(AppSet))
}
