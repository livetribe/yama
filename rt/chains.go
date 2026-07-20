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

package rt

import (
	"context"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/bridge"
)

// Chains holds the start, quiesce, and stop interceptor chains. It is built once,
// from the registered interceptors, and reused: WrapStart, WrapQuiesce, and
// WrapStop bind a component through these prebuilt chains without rebuilding them.
type Chains struct {
	start   []yama.StartInterceptor
	quiesce []yama.QuiesceInterceptor
	stop    []yama.StopInterceptor
}

// NewChains builds the three operation chains from interceptors. Each interceptor
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

// WrapStart binds component through the start chain, returning a Starter that
// attaches component identity to the context and runs the chain before invoking
// component.Start. It returns nil when component does not implement yama.Starter.
func (c *Chains) WrapStart(component any) yama.Starter {
	s, ok := component.(yama.Starter)
	if !ok {
		return nil
	}

	chain := s
	for i := len(c.start) - 1; i >= 0; i-- {
		chain = &startLink{interceptor: c.start[i], next: chain}
	}

	return &startEntry{component: component, chain: chain}
}

// WrapQuiesce binds component through the quiesce chain, returning a Quiescer that
// attaches component identity to the context and runs the chain before invoking
// component.Quiesce. It returns nil when component does not implement
// yama.Quiescer.
func (c *Chains) WrapQuiesce(component any) yama.Quiescer {
	q, ok := component.(yama.Quiescer)
	if !ok {
		return nil
	}

	chain := q
	for i := len(c.quiesce) - 1; i >= 0; i-- {
		chain = &quiesceLink{interceptor: c.quiesce[i], next: chain}
	}

	return &quiesceEntry{component: component, chain: chain}
}

// WrapStop binds component through the stop chain, returning a Stopper that
// attaches component identity to the context and runs the chain before invoking
// component.Stop. It returns nil when component does not implement yama.Stopper.
func (c *Chains) WrapStop(component any) yama.Stopper {
	s, ok := component.(yama.Stopper)
	if !ok {
		return nil
	}

	chain := s
	for i := len(c.stop) - 1; i >= 0; i-- {
		chain = &stopLink{interceptor: c.stop[i], next: chain}
	}

	return &stopEntry{component: component, chain: chain}
}

// startLink invokes one start interceptor with the rest of the chain as next.
type startLink struct {
	interceptor yama.StartInterceptor
	next        yama.Starter
}

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

// stopLink invokes one stop interceptor with the rest of the chain as next.
type stopLink struct {
	interceptor yama.StopInterceptor
	next        yama.Stopper
}

func (l *stopLink) Stop(ctx context.Context) {
	l.interceptor.Stop(ctx, l.next)
}

// startEntry attaches component identity to the context, then runs the start
// chain. It does not recover panics; a panic from the chain propagates.
type startEntry struct {
	component any
	chain     yama.Starter
}

func (e *startEntry) Start(ctx context.Context) error {
	return e.chain.Start(bridge.WithComponent(ctx, e.component))
}

// quiesceEntry attaches component identity to the context, then runs the quiesce
// chain. It does not recover panics; a panic from the chain propagates.
type quiesceEntry struct {
	component any
	chain     yama.Quiescer
}

func (e *quiesceEntry) Quiesce(ctx context.Context) {
	e.chain.Quiesce(bridge.WithComponent(ctx, e.component))
}

// stopEntry attaches component identity to the context, then runs the stop chain.
// It does not recover panics; a panic from the chain propagates.
type stopEntry struct {
	component any
	chain     yama.Stopper
}

func (e *stopEntry) Stop(ctx context.Context) {
	e.chain.Stop(bridge.WithComponent(ctx, e.component))
}
