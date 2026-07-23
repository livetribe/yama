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

package yama

import "context"

//go:generate go run go.uber.org/mock/mockgen -source=lifecycle.go -destination=internal/mocks/lifecycle_mocks.go -package=mocks

// The three capability interfaces below are optional and independent: a component
// may implement any combination of them, or none. Start returns error; Quiesce
// and Stop do not.

// Starter is implemented by components that participate in the startup pass.
type Starter interface {
	Start(context.Context) error
}

// Quiescer is implemented by components that participate in the quiesce pass —
// the first pass of shutdown, in which a component stops accepting new work and
// waits for its in-flight work to complete.
type Quiescer interface {
	Quiesce(context.Context)
}

// Stopper is implemented by components that participate in the teardown pass.
type Stopper interface {
	Stop(context.Context)
}

// Lifecycle is the primary lifecycle abstraction: applications drive startup and
// graceful shutdown through it. Applications do not implement or construct one;
// the generated lifecycle constructor returns it alongside the application:
//
//	app, lifecycle, err := NewLifecycle(WithInterceptors(i1, i2), WithBeginComponents(c1), WithEndComponents(c2))
//
// A Lifecycle starts and stops the whole graph, so it is itself a Starter and a
// Stopper — the same interfaces its components implement. It has no Quiesce:
// Stop runs the quiesce pass internally as its first action.
type Lifecycle interface {
	Starter
	Stopper
}
