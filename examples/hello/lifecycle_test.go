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

package hello_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"

	yama "l7e.io/yama"

	"example.com/hello"
)

// TestMain replaces the default slog logger with a discard handler for the
// whole run. Yama reports a skipped shutdown step through that logger. No
// test here asserts on log content.
func TestMain(m *testing.M) {
	discard := slog.New(slog.DiscardHandler)
	slog.SetDefault(discard)

	os.Exit(m.Run())
}

// TestLifecycleRunsInDependencyOrder starts and stops the generated lifecycle
// with an observer on every interceptor chain. The assertions check three
// facts. The levels start in dependency order. Shutdown walks the levels in
// reverse, with the quiesce pass first. The store's cleanup runs before the
// store's own Stop.
func TestLifecycleRunsInDependencyOrder(t *testing.T) {
	obs := &observer{}
	var out bytes.Buffer

	withObserver := yama.WithInterceptors(obs)
	_, lc, err := hello.NewLifecycle(&out, withObserver)
	if err != nil {
		t.Fatalf("NewLifecycle: %v", err)
	}

	if err := lc.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	started := obs.recorded()
	wantStarted := []string{"start store", "start server"}
	if !slices.Equal(started, wantStarted) {
		t.Fatalf("start events\n got %q\nwant %q", started, wantStarted)
	}

	lc.Stop(t.Context())

	all := obs.recorded()
	wantAll := []string{
		"start store",
		"start server",
		"quiesce server",
		"stop server",
		"stop store",
	}
	if !slices.Equal(all, wantAll) {
		t.Fatalf("lifecycle events\n got %q\nwant %q", all, wantAll)
	}

	const wantOutput = `store: open
server: serving "hello"
server: draining
server: stopped
store: released
store: closed
`
	if got := out.String(); got != wantOutput {
		t.Fatalf("component output\n got %q\nwant %q", got, wantOutput)
	}
}

// TestStartFailureReturnsErrStartFailedAlone fails the server's Start through
// an interceptor. The returned error must match ErrStartFailed and must not
// match the component's own error. An interceptor registered before the
// failing interceptor must see the component's own error. Start must tear
// down the started store before Start returns.
func TestStartFailureReturnsErrStartFailedAlone(t *testing.T) {
	cause := errors.New("the server did not start")
	seer := &errorSeer{}
	var out bytes.Buffer

	withInterceptors := yama.WithInterceptors(seer, &failServer{err: cause})
	_, lc, err := hello.NewLifecycle(&out, withInterceptors)
	if err != nil {
		t.Fatalf("NewLifecycle: %v", err)
	}

	err = lc.Start(t.Context())
	if !errors.Is(err, yama.ErrStartFailed) {
		t.Fatalf("Start returned %v; want an error that matches ErrStartFailed", err)
	}
	if errors.Is(err, cause) {
		t.Fatalf("Start returned the component's own error to the caller: %v", err)
	}
	if !errors.Is(seer.seen, cause) {
		t.Fatalf("the interceptor saw %v; want the component error %q", seer.seen, cause)
	}

	const wantOutput = `store: open
store: released
store: closed
`
	if got := out.String(); got != wantOutput {
		t.Fatalf("teardown output\n got %q\nwant %q", got, wantOutput)
	}
}

// observer is an interceptor that joins all three chains. It records one
// event for each component call, in arrival order.
type observer struct {
	mu     sync.Mutex
	events []string
}

// Start records the component's start, then calls next.
func (o *observer) Start(ctx context.Context, next yama.Starter) error {
	o.record(ctx, "start")

	return next.Start(ctx)
}

// Quiesce records the component's quiesce, then calls next.
func (o *observer) Quiesce(ctx context.Context, next yama.Quiescer) {
	o.record(ctx, "quiesce")

	next.Quiesce(ctx)
}

// Stop records the component's stop, then calls next.
func (o *observer) Stop(ctx context.Context, next yama.Stopper) {
	o.record(ctx, "stop")

	next.Stop(ctx)
}

// record appends one "<phase> <component>" event under the lock.
func (o *observer) record(ctx context.Context, phase string) {
	component, ok := yama.FromContext[fmt.Stringer](ctx)

	name := "unknown"
	if ok {
		name = component.String()
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, phase+" "+name)
}

// recorded returns a copy of the events under the lock.
func (o *observer) recorded() []string {
	o.mu.Lock()
	defer o.mu.Unlock()

	return slices.Clone(o.events)
}

// failServer is a start interceptor. It fails the server's Start with the
// injected error and calls next for every other component.
type failServer struct {
	err error
}

// Start returns the injected error for the server. For every other component,
// Start calls next.
func (f *failServer) Start(ctx context.Context, next yama.Starter) error {
	if _, ok := yama.FromContext[*hello.Server](ctx); ok {
		return f.err
	}

	return next.Start(ctx)
}

// errorSeer is a start interceptor. It keeps the error that next.Start
// returns for the server.
type errorSeer struct {
	seen error
}

// Start calls next and keeps the returned error when the component is the
// server.
func (s *errorSeer) Start(ctx context.Context, next yama.Starter) error {
	err := next.Start(ctx)

	if _, ok := yama.FromContext[*hello.Server](ctx); ok {
		s.seen = err
	}

	return err
}
