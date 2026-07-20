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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// TestChainOrderIsRegistrationOrder pins that a start chain runs its interceptors
// in registration order and reaches the component last.
func TestChainOrderIsRegistrationOrder(t *testing.T) {
	rec := &recorder{}
	interceptors := []any{
		&orderInterceptor{name: "Telemetry", rec: rec},
		&orderInterceptor{name: "Metrics", rec: rec},
		&orderInterceptor{name: "Logging", rec: rec},
	}
	comp := &fakeComponent{name: "component", rec: rec}

	err := rt.NewChains(interceptors).WrapStart(comp).Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, []string{"Telemetry", "Metrics", "Logging", "component"}, rec.list())
}

// TestChainOrderHoldsForEachOperation confirms quiesce and stop chains observe the
// same registration order.
func TestChainOrderHoldsForEachOperation(t *testing.T) {
	rec := &recorder{}
	interceptors := []any{
		&orderInterceptor{name: "Telemetry", rec: rec},
		&orderInterceptor{name: "Metrics", rec: rec},
		&orderInterceptor{name: "Logging", rec: rec},
	}
	comp := &fakeComponent{name: "component", rec: rec}
	chains := rt.NewChains(interceptors)

	chains.WrapQuiesce(comp).Quiesce(context.Background())
	assert.Equal(t, []string{"Telemetry", "Metrics", "Logging", "component"}, rec.list())

	rec2 := &recorder{}
	interceptors2 := []any{
		&orderInterceptor{name: "Telemetry", rec: rec2},
		&orderInterceptor{name: "Metrics", rec: rec2},
		&orderInterceptor{name: "Logging", rec: rec2},
	}
	comp2 := &fakeComponent{name: "component", rec: rec2}
	rt.NewChains(interceptors2).WrapStop(comp2).Stop(context.Background())
	assert.Equal(t, []string{"Telemetry", "Metrics", "Logging", "component"}, rec2.list())
}

// TestInterceptorsApplyGlobally proves one registered interceptor runs for every
// component wrapped through the same chains, with no per-component scoping.
func TestInterceptorsApplyGlobally(t *testing.T) {
	rec := &recorder{}
	chains := rt.NewChains([]any{&orderInterceptor{name: "global", rec: rec}})

	a := &fakeComponent{name: "a", rec: rec}
	b := &fakeComponent{name: "b", rec: rec}
	require.NoError(t, chains.WrapStart(a).Start(context.Background()))
	require.NoError(t, chains.WrapStart(b).Start(context.Background()))

	assert.Equal(t, []string{"global", "a", "global", "b"}, rec.list(),
		"the interceptor should run for every component, not a scoped subset")
}

// TestInterceptorJoinsOnlyMatchingChain proves a type implementing only
// StartInterceptor runs in the start chain and never in the stop chain.
func TestInterceptorJoinsOnlyMatchingChain(t *testing.T) {
	rec := &recorder{}
	startOnly := &startOnlyInterceptor{name: "start-only", rec: rec}
	comp := &fakeComponent{name: "component"}
	chains := rt.NewChains([]any{startOnly})

	err := chains.WrapStart(comp).Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"start-only"}, rec.list(), "start-only interceptor should run in the start chain")

	chains.WrapStop(comp).Stop(context.Background())
	assert.Equal(t, []string{"start-only"}, rec.list(), "start-only interceptor must not run in the stop chain")
}

// TestWrapReturnsNilForAbsentCapability confirms a component that lacks an
// operation is not wrapped for it.
func TestWrapReturnsNilForAbsentCapability(t *testing.T) {
	chains := rt.NewChains(nil)

	starter := &starterOnly{}
	assert.NotNil(t, chains.WrapStart(starter), "a Starter should wrap for start")
	assert.Nil(t, chains.WrapQuiesce(starter), "a non-Quiescer must not wrap for quiesce")
	assert.Nil(t, chains.WrapStop(starter), "a non-Stopper must not wrap for stop")

	stopper := &stopperOnly{}
	assert.Nil(t, chains.WrapStart(stopper), "a non-Starter must not wrap for start")
	assert.NotNil(t, chains.WrapStop(stopper), "a Stopper should wrap for stop")
}

// TestWrapperConcurrentInvocationIsRaceFree exercises the shared execution
// substrate: one wrapper invoked from many goroutines has no shared mutable state
// of its own, so every invocation reaches the component.
func TestWrapperConcurrentInvocationIsRaceFree(t *testing.T) {
	const goroutines = 50
	rec := &recorder{}
	comp := &fakeComponent{name: "component"}
	wrapped := rt.NewChains([]any{&orderInterceptor{name: "global", rec: rec}}).WrapStart(comp)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			assert.NoError(t, wrapped.Start(context.Background()))
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines, comp.starts(), "every concurrent invocation should reach the component")
	assert.Len(t, rec.list(), goroutines, "the interceptor should run once per invocation")
}

// spyInterceptor records the next value handed to it on each start invocation.
type spyInterceptor struct {
	nexts []yama.Starter
}

func (s *spyInterceptor) Start(ctx context.Context, next yama.Starter) error {
	s.nexts = append(s.nexts, next)
	return next.Start(ctx)
}

// TestChainsBuiltOnceAndReused proves the wrapper does not rebuild the chain per
// call: the interceptor sees the identical downstream chain object every time.
func TestChainsBuiltOnceAndReused(t *testing.T) {
	spy := &spyInterceptor{}
	comp := &fakeComponent{name: "component"}
	wrapped := rt.NewChains([]any{spy}).WrapStart(comp)

	const calls = 3
	for range calls {
		require.NoError(t, wrapped.Start(context.Background()))
	}

	require.Len(t, spy.nexts, calls)
	for i := 1; i < calls; i++ {
		assert.Same(t, spy.nexts[0], spy.nexts[i], "chain rebuilt between calls: downstream object differs")
	}
}
