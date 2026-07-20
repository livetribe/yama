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

package rt_test

import (
	"context"
	"sync"
	"time"

	yama "l7e.io/yama/v2"
)

// recorder collects an ordered, concurrency-safe log of events. Interceptors and
// components append to it to prove execution order.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, s)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// fakeComponent implements all three capabilities and a fmt.Stringer identity. Its
// per-operation behavior (recording, panicking, sleeping, error) is configurable.
type fakeComponent struct {
	name string
	rec  *recorder

	startErr     error
	startPanic   any
	quiescePanic any
	stopPanic    any
	sleep        time.Duration

	mu         sync.Mutex
	startCalls int
	completed  bool
	// startCtx is captured for assertion; the component does not act on it.
	startCtx context.Context
}

func (f *fakeComponent) Start(ctx context.Context) error {
	f.mu.Lock()
	f.startCalls++
	f.startCtx = ctx
	f.mu.Unlock()

	if f.rec != nil {
		f.rec.add(f.name)
	}
	if f.startPanic != nil {
		panic(f.startPanic)
	}
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}

	f.mu.Lock()
	f.completed = true
	f.mu.Unlock()
	return f.startErr
}

func (f *fakeComponent) Quiesce(_ context.Context) {
	if f.rec != nil {
		f.rec.add(f.name)
	}
	if f.quiescePanic != nil {
		panic(f.quiescePanic)
	}
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
}

func (f *fakeComponent) Stop(_ context.Context) {
	if f.rec != nil {
		f.rec.add(f.name)
	}
	if f.stopPanic != nil {
		panic(f.stopPanic)
	}
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
}

func (f *fakeComponent) String() string { return f.name }

func (f *fakeComponent) starts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCalls
}

func (f *fakeComponent) done() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.completed
}

func (f *fakeComponent) capturedStartCtx() context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCtx
}

// starterOnly implements only Starter and has no fmt.Stringer identity.
type starterOnly struct{}

func (*starterOnly) Start(context.Context) error { return nil }

// stopperOnly implements only Stopper and has no fmt.Stringer identity.
type stopperOnly struct{}

func (*stopperOnly) Stop(context.Context) {}

// orderInterceptor records its name in each operation, then delegates. It
// participates in all three chains.
type orderInterceptor struct {
	name string
	rec  *recorder
}

func (i *orderInterceptor) Start(ctx context.Context, next yama.Starter) error {
	i.rec.add(i.name)
	return next.Start(ctx)
}

func (i *orderInterceptor) Quiesce(ctx context.Context, next yama.Quiescer) {
	i.rec.add(i.name)
	next.Quiesce(ctx)
}

func (i *orderInterceptor) Stop(ctx context.Context, next yama.Stopper) {
	i.rec.add(i.name)
	next.Stop(ctx)
}

// startOnlyInterceptor implements only StartInterceptor.
type startOnlyInterceptor struct {
	name string
	rec  *recorder
}

func (i *startOnlyInterceptor) Start(ctx context.Context, next yama.Starter) error {
	i.rec.add(i.name)
	return next.Start(ctx)
}
