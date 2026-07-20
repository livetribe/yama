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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// traceKeyType is a private context key type, mirroring how an interceptor would
// carry tracing metadata without colliding with framework keys.
type traceKeyType struct{}

var traceKey traceKeyType

// observeInterceptor records the component identity it observes via FromContext.
type observeInterceptor struct{ observed any }

func (i *observeInterceptor) Start(ctx context.Context, next yama.Starter) error {
	i.observed, _ = yama.FromContext[any](ctx)
	return next.Start(ctx)
}

// tracingInterceptor attaches a span value to the context before calling next.
type tracingInterceptor struct{ span string }

func (i *tracingInterceptor) Start(ctx context.Context, next yama.Starter) error {
	return next.Start(context.WithValue(ctx, traceKey, i.span))
}

// suppressInterceptor returns without ever calling next.
type suppressInterceptor struct{}

func (suppressInterceptor) Start(context.Context, yama.Starter) error { return nil }

// swallowErrInterceptor invokes next but turns any error into nil.
type swallowErrInterceptor struct{}

func (swallowErrInterceptor) Start(ctx context.Context, next yama.Starter) error {
	_ = next.Start(ctx)
	return nil
}

// timingInterceptor measures how long next takes.
type timingInterceptor struct{ elapsed time.Duration }

func (i *timingInterceptor) Start(ctx context.Context, next yama.Starter) error {
	start := time.Now()
	err := next.Start(ctx)
	i.elapsed = time.Since(start)
	return err
}

// panicStartInterceptor panics instead of delegating.
type panicStartInterceptor struct{ value any }

func (i *panicStartInterceptor) Start(context.Context, yama.Starter) error {
	panic(i.value)
}

// TestInterceptorObservesComponent covers the observe capability: an interceptor
// reads the wrapped component's identity from context.
func TestInterceptorObservesComponent(t *testing.T) {
	obs := &observeInterceptor{}
	comp := &fakeComponent{name: "component"}

	err := rt.NewChains([]any{obs}).WrapStart(comp).Start(context.Background())
	require.NoError(t, err)
	assert.Same(t, comp, obs.observed, "interceptor should observe the wrapped component")
}

// TestInterceptorModifiesDownstreamContext covers the context-modification
// capability: a value an interceptor attaches, and the component identity, are
// both visible to the component downstream.
func TestInterceptorModifiesDownstreamContext(t *testing.T) {
	comp := &fakeComponent{name: "component"}

	err := rt.NewChains([]any{&tracingInterceptor{span: "span-123"}}).WrapStart(comp).Start(context.Background())
	require.NoError(t, err)

	ctx := comp.capturedStartCtx()
	require.NotNil(t, ctx)
	got, ok := yama.FromContext[*fakeComponent](ctx)
	require.True(t, ok, "component identity should reach the component downstream")
	assert.Same(t, comp, got)
	assert.Equal(t, "span-123", ctx.Value(traceKey), "interceptor-attached value should reach the component downstream")
}

// TestTracingMetadataSurvivesWrapper proves the wrapper neither strips nor
// replaces a value the caller attached before Start.
func TestTracingMetadataSurvivesWrapper(t *testing.T) {
	comp := &fakeComponent{name: "component"}
	ctx := context.WithValue(context.Background(), traceKey, "caller-span")

	err := rt.NewChains(nil).WrapStart(comp).Start(ctx)
	require.NoError(t, err)

	assert.Equal(t, "caller-span", comp.capturedStartCtx().Value(traceKey),
		"a caller-attached context value must survive the wrapper unchanged")
}

// TestInterceptorSuppressesExecution covers the suppress capability: an
// interceptor that returns without calling next prevents the component from
// running.
func TestInterceptorSuppressesExecution(t *testing.T) {
	comp := &fakeComponent{name: "component"}

	err := rt.NewChains([]any{suppressInterceptor{}}).WrapStart(comp).Start(context.Background())
	require.NoError(t, err)
	assert.Zero(t, comp.starts(), "suppressed component must not be invoked")
}

// TestInterceptorModifiesOutcome covers the outcome-modification capability: an
// interceptor turns a component's Start error into nil.
func TestInterceptorModifiesOutcome(t *testing.T) {
	comp := &fakeComponent{name: "component", startErr: errors.New("boom")}

	err := rt.NewChains([]any{swallowErrInterceptor{}}).WrapStart(comp).Start(context.Background())
	assert.NoError(t, err, "interceptor should turn the component error into nil")
	assert.Equal(t, 1, comp.starts(), "component should still have been invoked once")
}

// TestDurationMeasurableThroughWrapper proves a timing interceptor observes a
// duration covering the whole wrapped operation.
func TestDurationMeasurableThroughWrapper(t *testing.T) {
	const minWork = 20 * time.Millisecond
	timer := &timingInterceptor{}
	comp := &fakeComponent{name: "component", sleep: minWork}

	err := rt.NewChains([]any{timer}).WrapStart(comp).Start(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, timer.elapsed, minWork, "measured duration should cover the whole operation")
}

// TestUniversalWrappingWithoutInterceptors proves every component is wrapped even
// when no interceptor is attached: the component still runs, and its identity is
// still attached to the context.
func TestUniversalWrappingWithoutInterceptors(t *testing.T) {
	comp := &fakeComponent{name: "component"}
	wrapped := rt.NewChains(nil).WrapStart(comp)
	require.NotNil(t, wrapped, "component must be wrapped with no interceptors registered")

	require.NoError(t, wrapped.Start(context.Background()))
	assert.Equal(t, 1, comp.starts(), "component should be invoked exactly once")

	got, ok := yama.FromContext[*fakeComponent](comp.capturedStartCtx())
	require.True(t, ok, "identity should be attached even with no interceptors")
	assert.Same(t, comp, got)
}

// TestWrapperDoesNotRecoverStartComponentPanic proves the wrapper lets a component
// panic during Start propagate to its caller (Phase 3 owns recovery).
func TestWrapperDoesNotRecoverStartComponentPanic(t *testing.T) {
	comp := &fakeComponent{name: "component", startPanic: "start boom"}
	wrapped := rt.NewChains(nil).WrapStart(comp)

	assert.PanicsWithValue(t, "start boom", func() {
		_ = wrapped.Start(context.Background())
	})
}

// TestWrapperDoesNotRecoverStartInterceptorPanic proves an interceptor panic in
// the start chain propagates rather than being recovered.
func TestWrapperDoesNotRecoverStartInterceptorPanic(t *testing.T) {
	comp := &fakeComponent{name: "component"}
	wrapped := rt.NewChains([]any{&panicStartInterceptor{value: "interceptor boom"}}).WrapStart(comp)

	assert.PanicsWithValue(t, "interceptor boom", func() {
		_ = wrapped.Start(context.Background())
	})
	assert.Zero(t, comp.starts(), "component must not run when an outer interceptor panics")
}

// TestWrapperDoesNotRecoverQuiescePanic proves a quiesce panic propagates.
func TestWrapperDoesNotRecoverQuiescePanic(t *testing.T) {
	comp := &fakeComponent{name: "component", quiescePanic: "quiesce boom"}
	wrapped := rt.NewChains(nil).WrapQuiesce(comp)

	assert.PanicsWithValue(t, "quiesce boom", func() {
		wrapped.Quiesce(context.Background())
	})
}

// TestWrapperDoesNotRecoverStopPanic proves a stop panic propagates.
func TestWrapperDoesNotRecoverStopPanic(t *testing.T) {
	comp := &fakeComponent{name: "component", stopPanic: "stop boom"}
	wrapped := rt.NewChains(nil).WrapStop(comp)

	assert.PanicsWithValue(t, "stop boom", func() {
		wrapped.Stop(context.Background())
	})
}
