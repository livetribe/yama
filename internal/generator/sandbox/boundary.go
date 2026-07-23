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

package sandbox

import (
	"context"

	yama "l7e.io/yama/v2"
)

// The components below are standalone: they carry lifecycle interfaces but have
// no Wire providers, so no injector constructs them. The B* group and the E*
// group are shaped as begin- and end-boundary sets — components supplied to the
// Lifecycle directly rather than through the graph.

// The var block asserts, at compile time, which lifecycle interfaces each
// implements.
var (
	_ yama.Starter = (*B1)(nil)
	_ yama.Stopper = (*B1)(nil)

	_ yama.Stopper = (*B2)(nil)

	_ yama.Starter  = (*B3)(nil)
	_ yama.Quiescer = (*B3)(nil)
	_ yama.Stopper  = (*B3)(nil)

	_ yama.Quiescer = (*E1)(nil)
	_ yama.Stopper  = (*E1)(nil)

	_ yama.Starter  = (*E2)(nil)
	_ yama.Quiescer = (*E2)(nil)
	_ yama.Stopper  = (*E2)(nil)
)

// ---- begin boundary ----

// B1 implements Starter and Stopper.
type B1 struct{}

func (*B1) Start(context.Context) error { return nil }
func (*B1) Stop(context.Context)        {}

// B2 implements Stopper.
type B2 struct{}

func (*B2) Stop(context.Context) {}

// B3 implements Starter, Quiescer, and Stopper.
type B3 struct{}

func (*B3) Start(context.Context) error { return nil }
func (*B3) Quiesce(context.Context)     {}
func (*B3) Stop(context.Context)        {}

// ---- end boundary ----

// E1 implements Quiescer and Stopper.
type E1 struct{}

func (*E1) Quiesce(context.Context) {}
func (*E1) Stop(context.Context)    {}

// E2 implements Starter, Quiescer, and Stopper.
type E2 struct{}

func (*E2) Start(context.Context) error { return nil }
func (*E2) Quiesce(context.Context)     {}
func (*E2) Stop(context.Context)        {}
