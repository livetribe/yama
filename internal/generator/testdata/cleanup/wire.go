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
