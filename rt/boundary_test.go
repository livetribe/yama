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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// TestBoundaryBracketsAllPasses proves boundaries obey dependency ordering: begin
// is a base dependency (starts before the graph, tears down after it) and end is a
// top dependent (starts after the graph, tears down before it), so shutdown is the
// exact reverse of startup.
func TestBoundaryBracketsAllPasses(t *testing.T) {
	rec := &recorder{}
	beginC := &comp{name: "beginC", rec: rec}
	endC := &comp{name: "endC", rec: rec}
	db := &comp{name: "db", rec: rec}
	router := &comp{name: "router", rec: rec}
	worker := &comp{name: "worker", rec: rec}

	chains, begin, end := rt.Assemble(
		yama.WithBeginComponents(beginC),
		yama.WithEndComponents(endC),
	)
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.NoError(t, lc.Start(context.Background()))
	lc.Stop(context.Background())

	events := rec.list()

	// Start pass: begin before the base graph level, end after the last graph level.
	assertBefore(t, events, "beginC.start.exit", "db.start.enter")
	assertBefore(t, events, "router.start.exit", "endC.start.enter")
	assertBefore(t, events, "worker.start.exit", "endC.start.enter")

	// Quiesce pass reverses it: end runs before the graph, begin after it.
	assertBefore(t, events, "endC.quiesce.exit", "router.quiesce.enter")
	assertBefore(t, events, "endC.quiesce.exit", "worker.quiesce.enter")
	assertBefore(t, events, "db.quiesce.exit", "beginC.quiesce.enter")

	// Stop pass: same reversal.
	assertBefore(t, events, "endC.stop.exit", "router.stop.enter")
	assertBefore(t, events, "endC.stop.exit", "worker.stop.enter")
	assertBefore(t, events, "db.stop.exit", "beginC.stop.enter")
}

// TestNonStarterBoundaryScopedByReach proves a boundary non-starter is torn down
// only if its boundary was reached: on a graph startup failure the begin boundary
// (which ran first) is cleaned up, while the end boundary (never reached) is not.
func TestNonStarterBoundaryScopedByReach(t *testing.T) {
	beginStopper := &stopperComp{name: "beginStopper"}
	endStopper := &stopperComp{name: "endStopper"}
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", startErr: errors.New("worker boom")}

	chains, begin, end := rt.Assemble(
		yama.WithBeginComponents(beginStopper),
		yama.WithEndComponents(endStopper),
	)
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)

	assert.Equal(t, 1, beginStopper.stopN.get(), "a begin non-starter runs first, so it is reached and torn down")
	assert.Zero(t, endStopper.stopN.get(), "an end non-starter is never reached when the graph startup fails")
}

// TestBoundaryJoinsOnlyImplementedPass proves a boundary component participates in
// a pass only when it implements that pass's interface.
func TestBoundaryJoinsOnlyImplementedPass(t *testing.T) {
	beginStarter := &starterComp{name: "beginStarter"}
	endStopper := &stopperComp{name: "endStopper"}
	only := &comp{name: "only"}

	chains, begin, end := rt.Assemble(
		yama.WithBeginComponents(beginStarter),
		yama.WithEndComponents(endStopper),
	)
	lc := newSingle(chains, begin, end, only)

	require.NoError(t, lc.Start(context.Background()))
	require.NotPanics(t, func() { lc.Stop(context.Background()) })

	assert.Equal(t, 1, beginStarter.startN.get(), "a begin Starter joins the start pass")
	assert.Equal(t, 1, endStopper.stopN.get(), "an end Stopper joins the stop pass")
}

// TestBoundaryStartErrorFailsStartupLikeGraph proves a boundary Starter's error
// fails startup identically to a graph Starter's, for both begin and end
// placement.
func TestBoundaryStartErrorFailsStartupLikeGraph(t *testing.T) {
	t.Run("begin error stops the graph from starting", func(t *testing.T) {
		beginC := &comp{name: "beginC", startErr: errors.New("begin boom")}
		db := &comp{name: "db"}

		chains, begin, end := rt.Assemble(yama.WithBeginComponents(beginC))
		lc := newSingle(chains, begin, end, db)

		require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)
		assert.Zero(t, db.startN.get(), "a failing begin Starter must prevent the graph from starting")
	})

	t.Run("end error triggers cleanup of the started graph", func(t *testing.T) {
		endC := &comp{name: "endC", startErr: errors.New("end boom")}
		db := &comp{name: "db"}

		chains, begin, end := rt.Assemble(yama.WithEndComponents(endC))
		lc := newSingle(chains, begin, end, db)

		require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)
		assert.Equal(t, 1, db.stopN.get(), "a failing end Starter must clean up the started graph")
		assert.Zero(t, endC.stopN.get(), "the failed end component must not be torn down")
	})
}

// TestBoundaryStartPanicFailsStartupLikeGraph proves a boundary Starter's panic
// fails startup exactly as a graph Starter's would.
func TestBoundaryStartPanicFailsStartupLikeGraph(t *testing.T) {
	beginC := &comp{name: "beginC", startPanic: "begin panic"}
	db := &comp{name: "db"}

	chains, begin, end := rt.Assemble(yama.WithBeginComponents(beginC))
	lc := newSingle(chains, begin, end, db)

	var err error
	require.NotPanics(t, func() { err = lc.Start(context.Background()) })
	require.ErrorIs(t, err, yama.ErrStartFailed)
	assert.Zero(t, db.startN.get(), "a panicking begin Starter must prevent the graph from starting")
}

// TestBoundaryShutdownPanicRecoveredLikeGraph proves a boundary Quiescer or
// Stopper panic is recovered and swallowed so the pass completes, for both begin
// and end placement.
func TestBoundaryShutdownPanicRecoveredLikeGraph(t *testing.T) {
	beginC := &comp{name: "beginC", quiescePanic: "begin quiesce panic"}
	endC := &comp{name: "endC", stopPanic: "end stop panic"}
	db := &comp{name: "db"}

	chains, begin, end := rt.Assemble(
		yama.WithBeginComponents(beginC),
		yama.WithEndComponents(endC),
	)
	lc := newSingle(chains, begin, end, db)
	require.NoError(t, lc.Start(context.Background()))

	require.NotPanics(t, func() { lc.Stop(context.Background()) })
	assert.Equal(t, 1, db.stopN.get(), "the graph must still tear down after a recovered boundary panic")
}

// TestBoundaryInStartupFailureCleanup proves boundary components take part in
// startup-failure cleanup, scoped by the same started flags as graph components:
// a begin component that started is cleaned up, while an end component that the
// failing start never reached is not.
func TestBoundaryInStartupFailureCleanup(t *testing.T) {
	beginC := &comp{name: "beginC"}
	endC := &comp{name: "endC"}
	db := &comp{name: "db"}
	router := &comp{name: "router"}
	worker := &comp{name: "worker", startErr: errors.New("worker boom")}

	chains, begin, end := rt.Assemble(
		yama.WithBeginComponents(beginC),
		yama.WithEndComponents(endC),
	)
	lc := newDiamond(chains, begin, end, db, router, worker)
	require.ErrorIs(t, lc.Start(context.Background()), yama.ErrStartFailed)

	assert.Equal(t, 1, beginC.quiesceN.get(), "a started begin component is quiesced during cleanup")
	assert.Equal(t, 1, beginC.stopN.get(), "a started begin component is torn down during cleanup")
	assert.Equal(t, 1, db.stopN.get(), "the started dependency is torn down during cleanup")
	assert.Equal(t, 1, router.stopN.get(), "the started sibling is torn down during cleanup")
	assert.Zero(t, worker.stopN.get(), "the failed component is not torn down")
	assert.Zero(t, endC.stopN.get(), "an end component the failing start never reached is not torn down")
}

// TestBoundarySetIsConcurrentAndUnordered proves a boundary set's components run
// concurrently with no ordering between them.
func TestBoundarySetIsConcurrentAndUnordered(t *testing.T) {
	b := newBarrier()
	arrive := func(ctx context.Context) error {
		b.arrive(t)
		return nil
	}

	beginA := &comp{name: "beginA", onStart: arrive}
	beginB := &comp{name: "beginB", onStart: arrive}
	only := &comp{name: "only"}

	chains, begin, end := rt.Assemble(yama.WithBeginComponents(beginA, beginB))
	lc := newSingle(chains, begin, end, only)
	require.NoError(t, lc.Start(context.Background()))

	assert.Equal(t, 1, beginA.startN.get())
	assert.Equal(t, 1, beginB.startN.get())
}
