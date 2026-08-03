// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
