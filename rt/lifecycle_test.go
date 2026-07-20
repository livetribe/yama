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
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// assertBefore asserts the first event containing a precedes the first containing b.
func assertBefore(t *testing.T, events []string, a, b string) {
	t.Helper()

	ia := firstIndexContaining(events, a)
	ib := firstIndexContaining(events, b)
	require.GreaterOrEqualf(t, ia, 0, "event %q not found in %v", a, events)
	require.GreaterOrEqualf(t, ib, 0, "event %q not found in %v", b, events)

	assert.Lessf(t, ia, ib, "%q should occur before %q\nevents: %v", a, b, events)
}

// TestInvariant1_DependencyStartsBeforeDependent proves a dependency's Start
// completes before any dependent's Start begins, along the whole chain.
func TestInvariant1_DependencyStartsBeforeDependent(t *testing.T) {
	rec := &recorder{}
	base := &comp{name: "base", rec: rec}
	mid := &comp{name: "mid", rec: rec}
	top := &comp{name: "top", rec: rec}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)
	require.NoError(t, lc.Start(context.Background()))

	events := rec.list()
	assertBefore(t, events, "base.start.exit", "mid.start.enter")
	assertBefore(t, events, "mid.start.exit", "top.start.enter")
}

// TestSameLevelStartConcurrent proves independent same-level components start
// concurrently: a two-way barrier inside their Start releases only if both run at
// once.
func TestSameLevelStartConcurrent(t *testing.T) {
	b := newBarrier()
	arrive := func(ctx context.Context) error {
		b.arrive(t)
		return nil
	}

	db := &comp{name: "db"}
	router := &comp{name: "router", onStart: arrive}
	worker := &comp{name: "worker", onStart: arrive}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.NoError(t, lc.Start(context.Background()))
}

// runChainShutdown starts and stops a base → mid → top chain of full-capability
// components and returns the recorded events.
func runChainShutdown(t *testing.T) []string {
	t.Helper()

	rec := &recorder{}
	base := &comp{name: "base", rec: rec}
	mid := &comp{name: "mid", rec: rec}
	top := &comp{name: "top", rec: rec}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	return rec.list()
}

// TestInvariant2_DependentStopsBeforeDependency proves teardown runs in reverse
// dependency order.
func TestInvariant2_DependentStopsBeforeDependency(t *testing.T) {
	events := runChainShutdown(t)
	assertBefore(t, events, "top.stop.exit", "mid.stop.enter")
	assertBefore(t, events, "mid.stop.exit", "base.stop.enter")
}

// TestInvariant3_QuiesceBeforeAnyTeardown proves every component's Quiesce
// completes before any component's Stop begins.
func TestInvariant3_QuiesceBeforeAnyTeardown(t *testing.T) {
	events := runChainShutdown(t)

	lastQuiesce := lastIndexContaining(events, ".quiesce.")
	firstStop := firstIndexContaining(events, ".stop.")
	require.GreaterOrEqual(t, lastQuiesce, 0)
	require.GreaterOrEqual(t, firstStop, 0)
	assert.Lessf(t, lastQuiesce, firstStop,
		"no teardown may begin until every quiesce has completed\nevents: %v", events)
}

// TestQuiesceReverseDependencyOrder proves quiesce runs dependents-first, the same
// direction as teardown.
func TestQuiesceReverseDependencyOrder(t *testing.T) {
	events := runChainShutdown(t)
	assertBefore(t, events, "top.quiesce.exit", "mid.quiesce.enter")
	assertBefore(t, events, "mid.quiesce.exit", "base.quiesce.enter")
}

// TestQuiesceTransitiveThroughNonQuiescer proves quiesce ordering holds through a
// component that implements Starter and Stopper but not Quiescer: the two
// quiescing components stay ordered relative to each other.
func TestQuiesceTransitiveThroughNonQuiescer(t *testing.T) {
	rec := &recorder{}
	base := &comp{name: "base", rec: rec}
	mid := &startStopComp{name: "mid", rec: rec}
	top := &comp{name: "top", rec: rec}

	chains, begin, end := rt.Assemble()
	lc := newTransitive(chains, begin, end, base, mid, top)
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	events := rec.list()
	assertBefore(t, events, "top.quiesce.exit", "base.quiesce.enter")
	assert.Equal(t, 1, mid.startN.get(), "the non-Quiescer still starts")
	assert.Equal(t, 1, mid.stopN.get(), "the non-Quiescer still tears down")
	assert.Equal(t, -1, firstIndexContaining(events, "mid.quiesce"),
		"a non-Quiescer must never be quiesced")
}

// TestIndependentBranchesShutdownConcurrent proves independent same-level
// components quiesce concurrently, and tear down concurrently.
func TestIndependentBranchesShutdownConcurrent(t *testing.T) {
	cases := []struct {
		pass string
		set  func(router, worker *comp, arrive func(context.Context))
	}{
		{"quiesce", func(router, worker *comp, arrive func(context.Context)) {
			router.onQuiesce, worker.onQuiesce = arrive, arrive
		}},
		{"stop", func(router, worker *comp, arrive func(context.Context)) {
			router.onStop, worker.onStop = arrive, arrive
		}},
	}

	for _, tc := range cases {
		t.Run(tc.pass, func(t *testing.T) {
			b := newBarrier()
			arrive := func(ctx context.Context) { b.arrive(t) }

			db := &comp{name: "db"}
			router := &comp{name: "router"}
			worker := &comp{name: "worker"}
			tc.set(router, worker, arrive)

			chains, begin, end := rt.Assemble()
			lc := newDiamond(chains, begin, end, db, router, worker)
			require.NoError(t, lc.Start(context.Background()))
			lc.Stop(context.Background())
		})
	}
}

// TestCapabilitySubsets proves a Starter-only component receives only Start and a
// Stopper-only component receives only Stop.
func TestCapabilitySubsets(t *testing.T) {
	starter := &starterComp{name: "starter"}
	stopper := &stopperComp{name: "stopper"}

	chains, begin, end := rt.Assemble()
	lc := newCapability(chains, begin, end, starter, stopper)
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	assert.Equal(t, 1, starter.startN.get(), "Starter-only should start exactly once")
	assert.Equal(t, 1, stopper.stopN.get(), "Stopper-only should stop exactly once")
}

// TestNonStarterInUnreachedLevelNotTornDown proves a non-starter is torn down only
// if its level was reached: when an earlier level fails, a Stopper-only component
// in a later, never-reached level is left alone.
func TestNonStarterInUnreachedLevelNotTornDown(t *testing.T) {
	base := &comp{name: "base"}
	mid := &comp{name: "mid", startErr: errors.New("mid boom")}
	late := &stopperComp{name: "late"}

	chains, begin, end := rt.Assemble()
	lc := newReached(chains, begin, end, base, mid, late)
	require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)

	assert.Zero(t, late.stopN.get(), "a non-starter whose level was never reached must not be torn down")
	assert.Equal(t, 1, base.stopN.get(), "the started dependency is still cleaned up")
}

// TestStartupFailureCleanupScoped proves startup failure cleans up exactly the
// successfully-started components and skips the one that failed.
func TestStartupFailureCleanupScoped(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", startErr: errors.New("worker boom")}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	err := lc.Start(context.Background())
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Equal(t, 1, db.stopN.get(), "started dependency must be torn down")
	assert.Equal(t, 1, router.stopN.get(), "successfully-started sibling must be torn down")
	assert.Zero(t, worker.stopN.get(), "the component that failed to start must not be torn down")

	assert.Equal(t, 1, db.quiesceN.get(), "started dependency must be quiesced")
	assert.Equal(t, 1, router.quiesceN.get(), "successfully-started sibling must be quiesced")
	assert.Zero(t, worker.quiesceN.get(), "the component that failed to start must not be quiesced")
}

// TestStartupFailureStopsSchedulingLaterLevels proves a failure in one level
// prevents any later level from starting.
func TestStartupFailureStopsSchedulingLaterLevels(t *testing.T) {
	base := &comp{name: "base"}
	mid := &comp{name: "mid", startErr: errors.New("mid boom")}
	top := &comp{name: "top"}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)

	err := lc.Start(context.Background())
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Zero(t, top.startN.get(), "a level after the failing level must never start")
	assert.Equal(t, 1, base.stopN.get(), "the started dependency must be cleaned up")
	assert.Zero(t, top.stopN.get(), "an unstarted later component must not be cleaned up")
}

// TestStartupFailureCleanupIsOrderedShutdown proves startup-failure cleanup is the
// ordinary shutdown sequence: quiesce before teardown, both dependents-first.
func TestStartupFailureCleanupIsOrderedShutdown(t *testing.T) {
	rec := &recorder{}
	db := &comp{name: "db", rec: rec}
	router := &comp{name: "router", rec: rec}
	worker := &comp{name: "worker", rec: rec, startErr: errors.New("worker boom")}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)

	events := rec.list()
	lastQuiesce := lastIndexContaining(events, ".quiesce.")
	firstStop := firstIndexContaining(events, ".stop.")
	require.GreaterOrEqual(t, lastQuiesce, 0)
	require.GreaterOrEqual(t, firstStop, 0)
	assert.Less(t, lastQuiesce, firstStop, "cleanup must quiesce everything before any teardown")

	assertBefore(t, events, "router.quiesce.exit", "db.quiesce.enter")
	assertBefore(t, events, "router.stop.exit", "db.stop.enter")
}

// TestStartReturnsErrStartFailedNotComponentError proves Start surfaces only
// ErrStartFailed, never the component's own error.
func TestStartReturnsErrStartFailedNotComponentError(t *testing.T) {
	sentinel := errors.New("component-specific failure")
	db := &comp{name: "db", startErr: sentinel}

	chains, begin, end := rt.Assemble()
	lc := newSingle(chains, begin, end, db)

	err := lc.Start(context.Background())
	require.ErrorIs(t, err, yama.ErrStartFailed)
	assert.NotErrorIs(t, err, sentinel, "the component's error must not reach the caller")
}

// TestStartSuccessMeansComponentsStarted proves Start reports success only after
// actually starting components — nil is never a silent no-op.
func TestStartSuccessMeansComponentsStarted(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker"}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	require.NoError(t, lc.Start(context.Background()))
	assert.Equal(t, 1, db.startN.get())
	assert.Equal(t, 1, router.startN.get())
	assert.Equal(t, 1, worker.startN.get())
}

// TestStopIdempotentRepeated proves repeated Stop calls run the passes once.
func TestStopIdempotentRepeated(t *testing.T) {
	only := &comp{name: "only"}

	chains, begin, end := rt.Assemble()
	lc := newSingle(chains, begin, end, only)
	require.NoError(t, lc.Start(context.Background()))

	lc.Stop(context.Background())
	lc.Stop(context.Background())
	lc.Stop(context.Background())

	assert.Equal(t, 1, only.quiesceN.get(), "quiesce must run once across repeated Stop")
	assert.Equal(t, 1, only.stopN.get(), "teardown must run once across repeated Stop")
}

// TestStopIdempotentConcurrent proves concurrent Stop calls run the passes once
// and every caller observes completion.
func TestStopIdempotentConcurrent(t *testing.T) {
	only := &comp{name: "only"}

	chains, begin, end := rt.Assemble()
	lc := newSingle(chains, begin, end, only)
	require.NoError(t, lc.Start(context.Background()))

	const callers = 32
	done := make(chan struct{})
	for range callers {
		go func() {
			lc.Stop(context.Background())
			done <- struct{}{}
		}()
	}
	for range callers {
		<-done
	}

	assert.Equal(t, 1, only.quiesceN.get(), "quiesce must run once under concurrent Stop")
	assert.Equal(t, 1, only.stopN.get(), "teardown must run once under concurrent Stop")
}

// TestStopAfterStartupFailureCleanupIsNoOp proves a Stop that follows
// startup-failure cleanup does not re-run shutdown.
func TestStopAfterStartupFailureCleanupIsNoOp(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", startErr: errors.New("worker boom")}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)

	lc.Stop(context.Background())

	assert.Equal(t, 1, db.stopN.get(), "cleanup already tore down; Stop must not repeat it")
	assert.Equal(t, 1, router.stopN.get(), "cleanup already tore down; Stop must not repeat it")
}

// TestStartDeadlineExceeded proves a component that honors the caller's deadline
// and errors when it fires is treated as an ordinary start failure.
func TestStartDeadlineExceeded(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", onStart: honorCtx}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := lc.Start(ctx)
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Equal(t, 1, worker.startN.get(), "the deadline-honoring component ran and returned")
	assert.Zero(t, worker.stopN.get(), "the failed component must not be torn down")
	assert.Equal(t, 1, db.stopN.get(), "the started dependency must be cleaned up")
	assert.Equal(t, 1, router.stopN.get(), "the started sibling must be cleaned up")
}

// TestCallerContextAlreadyCanceled proves an already-canceled caller context makes
// startup fail, while any component that did start is cleaned up.
func TestCallerContextAlreadyCanceled(t *testing.T) {
	db := &comp{name: "db"} // ignores ctx; starts successfully
	router := &comp{name: "router", onStart: honorCtx}
	worker := &comp{name: "worker", onStart: honorCtx}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := lc.Start(ctx)
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Equal(t, 1, db.stopN.get(), "a component that did start must be cleaned up")
	assert.Zero(t, router.stopN.get(), "a component that honored cancellation never started")
	assert.Zero(t, worker.stopN.get(), "a component that honored cancellation never started")
}

// TestHungStartStallsTraversal proves the traversal waits on a component that
// never returns; only SIGKILL bounds this. The component is released on cleanup so
// the goroutine does not leak past the test.
func TestHungStartStallsTraversal(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	base := &comp{name: "base"}
	mid := &comp{name: "mid", onStart: func(ctx context.Context) error {
		<-release
		return nil
	}}
	top := &comp{name: "top"}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)

	returned := make(chan error, 1)
	go func() { returned <- lc.Start(context.Background()) }()

	select {
	case <-returned:
		t.Fatal("Start returned while a component was still running; the traversal must wait")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Zero(t, top.startN.get(), "a later component must not run while an earlier one is hung")
}

// TestHungStopStallsTraversal proves teardown waits on a component that never
// returns and does not proceed to its dependencies.
func TestHungStopStallsTraversal(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	base := &comp{name: "base"}
	mid := &comp{name: "mid"}
	top := &comp{name: "top", onStop: func(ctx context.Context) {
		<-release
	}}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)
	require.NoError(t, lc.Start(context.Background()))

	returned := make(chan struct{}, 1)
	go func() {
		lc.Stop(context.Background())
		returned <- struct{}{}
	}()

	select {
	case <-returned:
		t.Fatal("Stop returned while a component was still tearing down; the traversal must wait")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Zero(t, base.stopN.get(), "a dependency must not be torn down while a dependent is hung")
}

// TestStartComponentPanicRecovered proves a component panic during Start becomes
// ErrStartFailed with cleanup, not a propagating panic.
func TestStartComponentPanicRecovered(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", startPanic: "worker panic"}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	var err error
	require.NotPanics(t, func() { err = lc.Start(context.Background()) })
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Equal(t, 1, db.stopN.get(), "cleanup must run after a recovered start panic")
	assert.Equal(t, 1, router.stopN.get(), "cleanup must run after a recovered start panic")
	assert.Zero(t, worker.stopN.get(), "the panicking component never started")
}

// TestStartInterceptorPanicRecovered proves an interceptor panic during Start is
// recovered into ErrStartFailed with cleanup.
func TestStartInterceptorPanicRecovered(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker"}

	chains, begin, end := rt.Assemble(yama.WithInterceptors(&panicForComponent{target: "worker"}))
	lc := newDiamond(chains, begin, end, db, router, worker)

	var err error
	require.NotPanics(t, func() { err = lc.Start(context.Background()) })
	require.ErrorIs(t, err, yama.ErrStartFailed)

	assert.Equal(t, 1, db.stopN.get(), "cleanup must run after a recovered interceptor panic")
	assert.Equal(t, 1, router.stopN.get(), "cleanup must run after a recovered interceptor panic")
	assert.Zero(t, worker.startN.get(), "the component is never reached when its interceptor panics")
}

// TestStopComponentPanicRecovered proves a component panic during Stop is
// recovered and swallowed so the teardown traversal completes.
func TestStopComponentPanicRecovered(t *testing.T) {
	base := &comp{name: "base"}
	mid := &comp{name: "mid"}
	top := &comp{name: "top", stopPanic: "top stop panic"}

	chains, begin, end := rt.Assemble()
	lc := newChain(chains, begin, end, base, mid, top)
	require.NoError(t, lc.Start(context.Background()))

	require.NotPanics(t, func() { lc.Stop(context.Background()) })

	assert.Equal(t, 1, mid.stopN.get(), "teardown must continue past a recovered stop panic")
	assert.Equal(t, 1, base.stopN.get(), "the deepest dependency must still be torn down")
}

// TestZeroComponents proves an empty graph starts and stops cleanly.
func TestZeroComponents(t *testing.T) {
	chains, begin, end := rt.Assemble()
	_ = chains
	lc := newEmpty(begin, end)

	require.NoError(t, lc.Start(context.Background()))
	require.NotPanics(t, func() { lc.Stop(context.Background()) })
}

// TestSingleComponentAllCapabilities proves one full-capability component runs
// each pass exactly once, quiescing before teardown.
func TestSingleComponentAllCapabilities(t *testing.T) {
	rec := &recorder{}
	only := &comp{name: "only", rec: rec}

	chains, begin, end := rt.Assemble()
	lc := newSingle(chains, begin, end, only)
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	assert.Equal(t, 1, only.startN.get())
	assert.Equal(t, 1, only.quiesceN.get())
	assert.Equal(t, 1, only.stopN.get())
	assertBefore(t, rec.list(), "only.quiesce.exit", "only.stop.enter")
}

// TestCancelDuringShutdownStillCompletes proves shutdown runs both passes to
// completion even when the caller's context is already canceled.
func TestCancelDuringShutdownStillCompletes(t *testing.T) {
	only := &comp{name: "only"}

	chains, begin, end := rt.Assemble()
	lc := newSingle(chains, begin, end, only)
	require.NoError(t, lc.Start(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NotPanics(t, func() { lc.Stop(ctx) })

	assert.Equal(t, 1, only.quiesceN.get(), "quiesce runs to completion regardless of cancellation")
	assert.Equal(t, 1, only.stopN.get(), "teardown runs to completion regardless of cancellation")
}

// TestNoGoroutineLeak proves a full lifecycle leaves no lingering goroutines.
func TestNoGoroutineLeak(t *testing.T) {
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker"}

	chains, begin, end := rt.Assemble()
	lc := newDiamond(chains, begin, end, db, router, worker)

	before := runtime.NumGoroutine()
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	// Poll in the test's own goroutine so the check adds none of its own.
	settled := false
	for range 200 {
		if runtime.NumGoroutine() <= before {
			settled = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Truef(t, settled, "goroutines did not settle: before=%d after=%d", before, runtime.NumGoroutine())
}

// panicForComponent is a start interceptor that panics only for the component with
// the target identity, letting other components proceed.
type panicForComponent struct {
	target string
}

func (i *panicForComponent) Start(ctx context.Context, next yama.Starter) error {
	if c, ok := yama.FromContext[interface{ String() string }](ctx); ok && c.String() == i.target {
		panic("interceptor panic for " + i.target)
	}
	return next.Start(ctx)
}
