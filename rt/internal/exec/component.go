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
	"log/slog"

	"l7e.io/yama/v2"
)

// component holds one graph component's chain-bound operations. A nil operation
// means the component does not implement that capability, and the corresponding
// pass is a no-op for it.
type component struct {
	start   yama.Starter
	quiesce yama.Quiescer
	stop    yama.Stopper

	failed bool
}

var _ CompleteLifecycle = (*component)(nil)

// Start runs the component's Start and records whether it came up.
//
// failed is set before the call and cleared only on a clean return, so a Start
// that panics leaves it set. Assigning it after the call instead would mark a
// panicking component as started.
func (c *component) Start(ctx context.Context) error {
	if c.start == nil {
		return nil
	}

	c.failed = true

	if err := c.start.Start(ctx); err != nil {
		return err
	}

	c.failed = false

	return nil
}

// Quiesce runs the component's Quiesce, carrying the outcome of its Start so the
// gate can drop a component that never came up.
func (c *component) Quiesce(ctx context.Context) {
	if c.quiesce == nil {
		return
	}

	c.quiesce.Quiesce(withStatus(ctx, c.failed))
}

// Stop runs the component's Stop, carrying the outcome of its Start so the gate
// can drop a component that never came up.
func (c *component) Stop(ctx context.Context) {
	if c.stop == nil {
		return
	}

	c.stop.Stop(withStatus(ctx, c.failed))
}

// statusKey is the context key for the component status.
type statusKey struct{}

func withStatus(ctx context.Context, failed bool) context.Context {
	return context.WithValue(ctx, statusKey{}, failed)
}

func statusFromContext(ctx context.Context) (status, ok bool) {
	status, ok = ctx.Value(statusKey{}).(bool)
	return
}

// quiesceGate keeps a component whose Start failed out of the quiesce pass. It is
// the outermost link of the quiesce chain, so a component it skips reaches no
// interceptor, and nothing observes the skip but the log line.
type quiesceGate struct{}

// QuiesceGate is the gate every quiesce chain carries as its outermost link. The
// gate holds no state, so one instance serves every component.
var QuiesceGate yama.QuiesceInterceptor = &quiesceGate{}

func (e *quiesceGate) Quiesce(ctx context.Context, next yama.Quiescer) {
	if failed, ok := statusFromContext(ctx); ok && failed {
		slog.WarnContext(ctx, "component startup failed, skipping quiesce",
			slog.String("component", componentIdentity(ctx)),
		)

		return
	}

	next.Quiesce(ctx)
}

// stopGate keeps a component whose Start failed out of the teardown pass, on the
// same terms as quiesceGate.
//
// Google Wire cleanups are not gated, so a component that failed to start still
// releases what its provider acquired.
type stopGate struct{}

// StopGate is the gate every stop chain carries as its outermost link. The gate
// holds no state, so one instance serves every component.
var StopGate yama.StopInterceptor = &stopGate{}

func (e *stopGate) Stop(ctx context.Context, next yama.Stopper) {
	if failed, ok := statusFromContext(ctx); ok && failed {
		slog.WarnContext(ctx, "component startup failed, skipping stop",
			slog.String("component", componentIdentity(ctx)),
		)

		return
	}

	next.Stop(ctx)
}
