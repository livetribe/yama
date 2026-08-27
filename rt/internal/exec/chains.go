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

	"l7e.io/yama"
	"l7e.io/yama/internal/bridge"
)

// Chains holds the start, quiesce, and stop interceptor Chains. It is built once,
// from the registered interceptors, and reused: wrapStart, wrapQuiesce, and
// wrapStop bind a component through these prebuilt Chains without rebuilding them.
type Chains struct {
	start   []yama.StartInterceptor
	quiesce []yama.QuiesceInterceptor
	stop    []yama.StopInterceptor
}

// NewChains builds the three operation Chains from interceptors. Each interceptor
// joins an operation's chain only when it implements that operation's interceptor
// interface, and registration order is preserved. The built-in overrun
// interceptor is appended to each chain as its innermost element.
func NewChains(interceptors []any) *Chains {
	c := &Chains{}

	for _, i := range interceptors {
		if s, ok := i.(yama.StartInterceptor); ok {
			c.start = append(c.start, s)
		}
		if q, ok := i.(yama.QuiesceInterceptor); ok {
			c.quiesce = append(c.quiesce, q)
		}
		if s, ok := i.(yama.StopInterceptor); ok {
			c.stop = append(c.stop, s)
		}
	}

	c.start = append(c.start, overrun)
	c.quiesce = append(c.quiesce, overrun)
	c.stop = append(c.stop, overrun)

	return c
}

// WrapComponent binds a component through the Chains for each capability it
// implements. A capability the component lacks is left nil, making that pass a
// no-op for it, so the result participates in exactly the passes the component
// declared and can be treated uniformly by the level that holds it.
func (c *Chains) WrapComponent(a any) CompleteLifecycle {
	comp := &component{}

	if s, ok := a.(yama.Starter); ok {
		comp.start = c.wrapStart(s)
	}
	if q, ok := a.(yama.Quiescer); ok {
		comp.quiesce = c.wrapQuiesce(q)
	}
	if s, ok := a.(yama.Stopper); ok {
		comp.stop = c.wrapStop(s)
	}

	return comp
}

// wrapStart binds component through the start chain, returning a Starter that
// attaches component identity to the context and runs the chain before invoking
// component.Start.
func (c *Chains) wrapStart(component yama.Starter) yama.Starter {
	chain := component
	for i := len(c.start) - 1; i >= 0; i-- {
		chain = &startLink{interceptor: c.start[i], next: chain}
	}

	return &startEntry{component: component, chain: chain}
}

// wrapQuiesce binds component through the quiesce chain, returning a Quiescer that
// attaches component identity to the context and runs the chain before invoking
// component.Quiesce. quiesceGate is added outermost, ahead of every registered
// interceptor.
func (c *Chains) wrapQuiesce(component yama.Quiescer) yama.Quiescer {
	chain := component
	for i := len(c.quiesce) - 1; i >= 0; i-- {
		chain = &quiesceLink{interceptor: c.quiesce[i], next: chain}
	}

	chain = &quiesceLink{interceptor: QuiesceGate, next: chain}

	return &quiesceEntry{component: component, chain: chain}
}

// wrapStop binds component through the stop chain, returning a Stopper that
// attaches component identity to the context and runs the chain before invoking
// component.Stop. stopGate is added outermost, ahead of every registered
// interceptor.
func (c *Chains) wrapStop(component yama.Stopper) yama.Stopper {
	chain := component
	for i := len(c.stop) - 1; i >= 0; i-- {
		chain = &stopLink{interceptor: c.stop[i], next: chain}
	}

	chain = &stopLink{interceptor: StopGate, next: chain}

	return &stopEntry{component: component, chain: chain}
}

// startLink invokes one start interceptor with the rest of the chain as next.
type startLink struct {
	interceptor yama.StartInterceptor
	next        yama.Starter
}

var _ yama.Starter = (*startLink)(nil)

func (l *startLink) Start(ctx context.Context) error {
	return l.interceptor.Start(ctx, l.next)
}

// quiesceLink invokes one quiesce interceptor with the rest of the chain as next.
type quiesceLink struct {
	interceptor yama.QuiesceInterceptor
	next        yama.Quiescer
}

func (l *quiesceLink) Quiesce(ctx context.Context) {
	l.interceptor.Quiesce(ctx, l.next)
}

var _ yama.Quiescer = (*quiesceLink)(nil)

// stopLink invokes one stop interceptor with the rest of the chain as next.
type stopLink struct {
	interceptor yama.StopInterceptor
	next        yama.Stopper
}

var _ yama.Stopper = (*stopLink)(nil)

func (l *stopLink) Stop(ctx context.Context) {
	l.interceptor.Stop(ctx, l.next)
}

// startEntry attaches component identity to the context, then runs the start
// chain. It does not recover panics; a panic from the chain propagates.
type startEntry struct {
	component any
	chain     yama.Starter
}

var _ yama.Starter = (*startEntry)(nil)

func (e *startEntry) Start(ctx context.Context) error {
	return e.chain.Start(bridge.WithComponent(ctx, e.component))
}

// quiesceEntry attaches component identity to the context, then runs the quiesce
// chain. It does not recover panics; a panic from the chain propagates.
type quiesceEntry struct {
	component any
	chain     yama.Quiescer
}

var _ yama.Quiescer = (*quiesceEntry)(nil)

func (e *quiesceEntry) Quiesce(ctx context.Context) {
	e.chain.Quiesce(bridge.WithComponent(ctx, e.component))
}

// stopEntry attaches component identity to the context, then runs the stop chain.
// It does not recover panics; a panic from the chain propagates.
type stopEntry struct {
	component any
	chain     yama.Stopper
}

var _ yama.Stopper = (*stopEntry)(nil)

func (e *stopEntry) Stop(ctx context.Context) {
	e.chain.Stop(bridge.WithComponent(ctx, e.component))
}
