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

package exec

import (
	"context"
)

// A Cleanup is the Google Wire cleanup function of a value that implements no
// lifecycle capability of its own. It occupies that value's place in its level,
// so the cleanup runs at the value's position in the ordering.
//
// A Wire cleanup is a backward-compatibility mechanism rather than a lifecycle
// capability: it takes no context and carries no identity, and it does not pass
// through the interceptor chains.
type Cleanup func()

var _ CompleteLifecycle = Cleanup(nil)

// Start does nothing and always succeeds; a cleanup has no startup work.
func (fn Cleanup) Start(_ context.Context) error {
	return nil
}

// Quiesce does nothing; a cleanup has no quiesce work.
func (fn Cleanup) Quiesce(_ context.Context) {}

// Stop runs the cleanup. It runs whether or not the level was reached during a
// failed startup, since a cleanup carries no start outcome to gate on.
func (fn Cleanup) Stop(_ context.Context) {
	fn()
}

// cleanableComponent pairs a lifecycle-capable component with the Google Wire
// cleanup function its provider returned. The pair occupies the cleaned-up
// value's place in its level, so the cleanup inherits that value's ordering
// against the rest of the graph.
//
// A Wire cleanup is a backward-compatibility mechanism rather than a lifecycle
// capability: it takes no context and carries no identity, and it does not pass
// through the interceptor chains.
type cleanableComponent struct {
	component CompleteLifecycle
	cleanup   func()
}

var _ CompleteLifecycle = (*cleanableComponent)(nil)

// NewCleanableComponent pairs c with the cleanup its provider returned. Use
// Cleanup instead when the value implements no lifecycle capability.
func NewCleanableComponent(c CompleteLifecycle, fn func()) CompleteLifecycle {
	return &cleanableComponent{
		component: c,
		cleanup:   fn,
	}
}

func (c *cleanableComponent) Start(ctx context.Context) error {
	return c.component.Start(ctx)
}

func (c *cleanableComponent) Quiesce(ctx context.Context) {
	c.component.Quiesce(ctx)
}

// Stop releases the provider's resources and then tears the component down.
//
// The order between the two is fixed but arbitrary: a cleanup can release what
// the component's Stop still needs, and a cleanup can release what its Stop is
// waiting on, so neither order is safe in general. A provider that both returns a
// cleanup and produces a Stopper owns the interaction between them.
//
// The cleanup runs whether or not the component started, since it sits outside
// the gate that drops a failed component's teardown.
func (c *cleanableComponent) Stop(ctx context.Context) {
	c.cleanup()
	c.component.Stop(ctx)
}
