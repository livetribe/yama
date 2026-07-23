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

// cleanup pairs a component with the Google Wire cleanup function its provider
// returned. The pair occupies the cleaned-up value's place in its level, so the
// cleanup inherits that value's ordering against the rest of the graph.
//
// A Wire cleanup is a backward-compatibility mechanism rather than a lifecycle
// capability: it takes no context and carries no identity, and it does not pass
// through the interceptor chains.
type cleanup struct {
	component CompleteLifecycle
	cleanup   func()
}

var _ CompleteLifecycle = (*cleanup)(nil)

func NewCleanup(c CompleteLifecycle, fn func()) CompleteLifecycle {
	return &cleanup{
		component: c,
		cleanup:   fn,
	}
}

func (c *cleanup) Start(ctx context.Context) error {
	return c.component.Start(ctx)
}

func (c *cleanup) Quiesce(ctx context.Context) {
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
func (c *cleanup) Stop(ctx context.Context) {
	c.cleanup()
	c.component.Stop(ctx)
}
