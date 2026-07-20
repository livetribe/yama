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
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	yama "l7e.io/yama/v2"
)

// Flag is one lifecycle component's started flag. Its zero value is unset and
// ready to use. A start Group sets it when the component's Start returns without
// error or panic; the shutdown groups read it to scope quiesce and teardown to
// components that actually started.
type Flag struct {
	started atomic.Bool
}

func (f *Flag) set() {
	f.started.Store(true)
}

func (f *Flag) get() bool {
	return f.started.Load()
}

// Started is the start member for a component that implements no Starter. Handed
// to Group.Go, it marks that component's started flag when its level runs, so a
// non-Starter counts as started exactly when its level is reached. It runs no
// interceptor chain and attaches no identity: a component with no Start method has
// no start notification to observe.
var Started yama.Starter = noopStarter{}

type noopStarter struct{}

func (noopStarter) Start(context.Context) error {
	return nil
}

// panicError carries a recovered panic value and the stack captured at recovery,
// so a panic converted to a start failure remains diagnosable. It never reaches a
// lifecycle caller: Start reports only ErrStartFailed.
type panicError struct {
	value any
	stack []byte
}

func (e *panicError) Error() string {
	return fmt.Sprintf("recovered panic: %v\n%s", e.value, e.stack)
}

// Group runs a start level's member components concurrently with fail-fast. The
// first member to fail cancels the group's context so in-flight members observe
// cancellation, and Wait returns once every launched member has settled. A member
// that panics is recovered and treated as a failure.
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	mu  sync.Mutex
	err error
}

// NewGroup returns a Group whose members run under a cancelable child of parent.
func NewGroup(parent context.Context) *Group {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}
}

// Go launches s.Start concurrently. On success, it sets started; on failure or
// panic it records the first failure and cancels the group. A nil started is
// permitted and skips the flag update.
func (g *Group) Go(s yama.Starter, started *Flag) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		if err := recoverStart(g.ctx, s); err != nil {
			g.fail(err)
			return
		}

		if started != nil {
			started.set()
		}
	}()
}

func (g *Group) fail(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.err == nil {
		g.err = err
		g.cancel()
	}
}

// Wait blocks until every launched member has settled and returns the first
// failure, or nil if all members succeeded.
func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()
	return g.err
}

// recoverStart runs s.Start, converting a panic into an error so the start level
// fails fast rather than propagating the panic.
func recoverStart(ctx context.Context, s yama.Starter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &panicError{value: r, stack: debug.Stack()}
		}
	}()

	return s.Start(ctx)
}

// ShutdownGroup runs a quiesce or teardown level's member operations
// concurrently. It has no fail-fast: every eligible member runs, a panic is
// recovered and swallowed, and Wait blocks until all have returned. A member
// whose started flag is unset is skipped.
type ShutdownGroup struct {
	ctx context.Context
	wg  sync.WaitGroup
}

// NewShutdownGroup returns a ShutdownGroup whose members run under ctx unchanged.
func NewShutdownGroup(ctx context.Context) *ShutdownGroup {
	return &ShutdownGroup{ctx: ctx}
}

// Go launches run concurrently when started is nil or set; otherwise the member
// never started and is skipped. A panic from run is recovered and swallowed so
// the traversal completes.
func (g *ShutdownGroup) Go(run func(context.Context), started *Flag) {
	if started != nil && !started.get() {
		return
	}

	g.wg.Add(1)

	go func() {
		defer g.wg.Done()
		defer func() { _ = recover() }()

		run(g.ctx)
	}()
}

// Wait blocks until every launched member has returned.
func (g *ShutdownGroup) Wait() {
	g.wg.Wait()
}
