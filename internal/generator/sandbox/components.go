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
	"fmt"

	yama "l7e.io/yama/v2"
)

// The var blocks below assert, at compile time, which lifecycle interfaces each
// component implements — the capabilities annotated on the graph diagram.
var (
	_ yama.Stopper = (*Base1)(nil)

	_ yama.Starter  = (*Base3)(nil)
	_ yama.Quiescer = (*Base3)(nil)
	_ yama.Stopper  = (*Base3)(nil)

	_ yama.Starter  = (*Mid2)(nil)
	_ yama.Quiescer = (*Mid2)(nil)
	_ yama.Stopper  = (*Mid2)(nil)

	_ yama.Starter = (*Root2)(nil)
	_ yama.Stopper = (*Root2)(nil)

	_ yama.Starter  = (*Root3)(nil)
	_ yama.Quiescer = (*Root3)(nil)
	_ yama.Stopper  = (*Root3)(nil)
)

// ---- base layer ----

// Base1 implements Stopper.
type Base1 struct{}

func NewBase1(log Logger) *Base1 {
	log.Logf("construct base1")
	return &Base1{}
}

func (*Base1) Stop(context.Context) {}
func (*Base1) tree() string         { return "base1" }

// Base2 implements no lifecycle interface. Its provider returns a cleanup
// function and an error, so Wire threads Base2 through the injector's aggregated
// cleanup and error paths.
type Base2 struct{}

func NewBase2(log Logger) (*Base2, func(), error) {
	log.Logf("construct base2")

	cleanup := func() { log.Logf("cleanup base2") }

	return &Base2{}, cleanup, nil
}

func (*Base2) tree() string { return "base2" }

// Base3 implements Starter, Quiescer, and Stopper. Its Environment argument is
// supplied by wire.FieldsOf projecting Config.Env. Its provider returns a cleanup
// function without an error — the other of the two cleanup shapes Wire folds into
// the injector's aggregated cleanup (Base2 returns cleanup and error).
type Base3 struct{}

func NewBase3(log Logger, env Environment) (base *Base3, cleanup func()) {
	log.Logf("construct base3 (env=%s)", env)

	cleanup = func() { log.Logf("cleanup base3") }

	return &Base3{}, cleanup
}

func (*Base3) Start(context.Context) error { return nil }
func (*Base3) Quiesce(context.Context)     {}
func (*Base3) Stop(context.Context)        {}
func (*Base3) tree() string                { return "base3" }

// ---- mid layer ----

// Mid1 implements no lifecycle interface.
type Mid1 struct {
	base1 *Base1
	base2 *Base2
}

func NewMid1(log Logger, base1 *Base1, base2 *Base2) *Mid1 {
	log.Logf("construct mid1")
	return &Mid1{base1: base1, base2: base2}
}

func (m *Mid1) tree() string {
	return fmt.Sprintf("mid1 → [%s, %s]", m.base1.tree(), m.base2.tree())
}

// Mid2 implements Starter, Quiescer, and Stopper.
type Mid2 struct {
	base2 *Base2
	base3 *Base3
}

func NewMid2(log Logger, base2 *Base2, base3 *Base3) *Mid2 {
	log.Logf("construct mid2")
	return &Mid2{base2: base2, base3: base3}
}

func (*Mid2) Start(context.Context) error { return nil }
func (*Mid2) Quiesce(context.Context)     {}
func (*Mid2) Stop(context.Context)        {}

func (m *Mid2) tree() string {
	return fmt.Sprintf("mid2 → [%s, %s]", m.base2.tree(), m.base3.tree())
}

// ---- root layer ----

// Root1 implements no lifecycle interface.
type Root1 struct {
	mid1 *Mid1
}

func NewRoot1(log Logger, mid1 *Mid1) *Root1 {
	log.Logf("construct root1")
	return &Root1{mid1: mid1}
}

func (r *Root1) tree() string {
	return "root1 → " + r.mid1.tree()
}

// Root2 implements Starter and Stopper.
type Root2 struct {
	mid1 *Mid1
}

func NewRoot2(log Logger, mid1 *Mid1) *Root2 {
	log.Logf("construct root2")
	return &Root2{mid1: mid1}
}

func (*Root2) Start(context.Context) error { return nil }
func (*Root2) Stop(context.Context)        {}

func (r *Root2) tree() string {
	return "root2 → " + r.mid1.tree()
}

// Root3 implements Starter, Quiescer, and Stopper.
type Root3 struct {
	mid2 *Mid2
}

func NewRoot3(log Logger, mid2 *Mid2) *Root3 {
	log.Logf("construct root3")
	return &Root3{mid2: mid2}
}

func (*Root3) Start(context.Context) error { return nil }
func (*Root3) Quiesce(context.Context)     {}
func (*Root3) Stop(context.Context)        {}

func (r *Root3) tree() string {
	return "root3 → " + r.mid2.tree()
}

// App is the graph's root object. Wire fills its fields with wire.Struct.
type App struct {
	root1 *Root1
	root2 *Root2
	root3 *Root3
}

// String renders the wired dependency tree, reading every edge Wire populated.
func (a *App) String() string {
	return fmt.Sprintf("App\n  ├─ %s\n  ├─ %s\n  └─ %s",
		a.root1.tree(), a.root2.tree(), a.root3.tree())
}
