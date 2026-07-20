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

// The types below are hand-authored stand-ins for the code the generator emits.
// They match the shape pinned in internal/generator/testdata/skeleton.go.example:
// generated levels declare per-member wrapped-operation fields and a started flag,
// and their Start/Quiesce/Stop methods drive rt.NewGroup / rt.NewShutdownGroup;
// the graph's startPass/shutdown methods encode ordering directly with explicit
// per-level calls, never a loop over a runtime level list. rt.NewLifecycle wraps
// the two graph methods into a yama.Lifecycle.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// honorCtx blocks until ctx is done, then reports its error. A start fixture uses
// it to model a component that honors the caller's deadline or cancellation
// instead of ignoring it.
func honorCtx(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// comp is a lifecycle component implementing all three capabilities and a
// fmt.Stringer identity. It records enter/exit events per operation, counts
// invocations, and takes per-operation hooks and panic values so one type covers
// ordering, concurrency, failure, and panic scenarios.
type comp struct {
	name string
	rec  *recorder

	startErr error

	startPanic   any
	quiescePanic any
	stopPanic    any

	onStart   func(ctx context.Context) error
	onQuiesce func(ctx context.Context)
	onStop    func(ctx context.Context)

	startN   counter
	quiesceN counter
	stopN    counter
}

func (c *comp) Start(ctx context.Context) error {
	c.startN.inc()
	c.event("start", "enter")

	if c.startPanic != nil {
		panic(c.startPanic)
	}

	err := c.startErr
	if c.onStart != nil {
		err = c.onStart(ctx)
	}

	c.event("start", "exit")
	return err
}

func (c *comp) Quiesce(ctx context.Context) {
	c.quiesceN.inc()
	c.event("quiesce", "enter")

	if c.quiescePanic != nil {
		panic(c.quiescePanic)
	}
	if c.onQuiesce != nil {
		c.onQuiesce(ctx)
	}

	c.event("quiesce", "exit")
}

func (c *comp) Stop(ctx context.Context) {
	c.stopN.inc()
	c.event("stop", "enter")

	if c.stopPanic != nil {
		panic(c.stopPanic)
	}
	if c.onStop != nil {
		c.onStop(ctx)
	}

	c.event("stop", "exit")
}

func (c *comp) String() string {
	return c.name
}

func (c *comp) event(op, phase string) {
	if c.rec != nil {
		c.rec.add(c.name + "." + op + "." + phase)
	}
}

// starterComp implements only Starter.
type starterComp struct {
	name   string
	rec    *recorder
	startN counter
}

func (c *starterComp) Start(context.Context) error {
	c.startN.inc()
	if c.rec != nil {
		c.rec.add(c.name + ".start")
	}
	return nil
}

// stopperComp implements only Stopper.
type stopperComp struct {
	name  string
	rec   *recorder
	stopN counter
}

func (c *stopperComp) Stop(context.Context) {
	c.stopN.inc()
	if c.rec != nil {
		c.rec.add(c.name + ".stop")
	}
}

// startStopComp implements Starter and Stopper but not Quiescer. It models a
// lifecycle component that transmits ordering through the quiesce pass without
// participating in it.
type startStopComp struct {
	name   string
	rec    *recorder
	startN counter
	stopN  counter
}

func (c *startStopComp) Start(context.Context) error {
	c.startN.inc()
	if c.rec != nil {
		c.rec.add(c.name + ".start.enter")
		c.rec.add(c.name + ".start.exit")
	}
	return nil
}

func (c *startStopComp) Stop(context.Context) {
	c.stopN.inc()
	if c.rec != nil {
		c.rec.add(c.name + ".stop.enter")
		c.rec.add(c.name + ".stop.exit")
	}
}

// counter is a concurrency-safe invocation count.
type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// barrier proves concurrency: two operations that each call arrive proceed only
// once both have arrived. If the operations run serially, arrive times out and
// fails the test rather than deadlocking the suite.
type barrier struct {
	ready chan struct{}

	mu    sync.Mutex
	count int
}

func newBarrier() *barrier {
	return &barrier{ready: make(chan struct{})}
}

func (b *barrier) arrive(t *testing.T) {
	t.Helper()

	b.mu.Lock()
	b.count++
	if b.count == 2 {
		close(b.ready)
	}
	b.mu.Unlock()

	select {
	case <-b.ready:
	case <-time.After(2 * time.Second):
		t.Errorf("barrier reached %d of 2: operations did not run concurrently", b.count)
	}
}

// soloLevel is a generated level with one full-capability member.
type soloLevel struct {
	start   yama.Starter
	quiesce yama.Quiescer
	stop    yama.Stopper
	started rt.Flag
}

func solo(chains *rt.Chains, component any) *soloLevel {
	return &soloLevel{
		start:   chains.WrapStart(component),
		quiesce: chains.WrapQuiesce(component),
		stop:    chains.WrapStop(component),
	}
}

func (lvl *soloLevel) Start(ctx context.Context) error {
	g := rt.NewGroup(ctx)
	g.Go(lvl.start, &lvl.started)
	return g.Wait()
}

func (lvl *soloLevel) Quiesce(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.quiesce.Quiesce, &lvl.started)
	g.Wait()
}

func (lvl *soloLevel) Stop(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.stop.Stop, &lvl.started)
	g.Wait()
}

// pairLevel is a generated level with two independent full-capability members.
type pairLevel struct {
	startA   yama.Starter
	quiesceA yama.Quiescer
	stopA    yama.Stopper
	startedA rt.Flag

	startB   yama.Starter
	quiesceB yama.Quiescer
	stopB    yama.Stopper
	startedB rt.Flag
}

func pair(chains *rt.Chains, a, b any) *pairLevel {
	return &pairLevel{
		startA:   chains.WrapStart(a),
		quiesceA: chains.WrapQuiesce(a),
		stopA:    chains.WrapStop(a),
		startB:   chains.WrapStart(b),
		quiesceB: chains.WrapQuiesce(b),
		stopB:    chains.WrapStop(b),
	}
}

func (lvl *pairLevel) Start(ctx context.Context) error {
	g := rt.NewGroup(ctx)
	g.Go(lvl.startA, &lvl.startedA)
	g.Go(lvl.startB, &lvl.startedB)
	return g.Wait()
}

func (lvl *pairLevel) Quiesce(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.quiesceA.Quiesce, &lvl.startedA)
	g.Go(lvl.quiesceB.Quiesce, &lvl.startedB)
	g.Wait()
}

func (lvl *pairLevel) Stop(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.stopA.Stop, &lvl.startedA)
	g.Go(lvl.stopB.Stop, &lvl.startedB)
	g.Wait()
}

// startStopLevel is a generated level whose single member implements Starter and
// Stopper but not Quiescer, so it has no Quiesce method — it never joins the
// quiesce pass while still bracketing the levels around it.
type startStopLevel struct {
	start   yama.Starter
	stop    yama.Stopper
	started rt.Flag
}

func startStop(chains *rt.Chains, component any) *startStopLevel {
	return &startStopLevel{
		start: chains.WrapStart(component),
		stop:  chains.WrapStop(component),
	}
}

func (lvl *startStopLevel) Start(ctx context.Context) error {
	g := rt.NewGroup(ctx)
	g.Go(lvl.start, &lvl.started)
	return g.Wait()
}

func (lvl *startStopLevel) Stop(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.stop.Stop, &lvl.started)
	g.Wait()
}

// mixedLevel is a generated level holding a Starter-only member and a
// Stopper-only member. Each carries a started flag: the Starter-only member's is
// set when its Start succeeds; the Stopper-only member's is marked via rt.Started
// when the level is reached. Its teardown gates on that flag.
type mixedLevel struct {
	start          yama.Starter
	starterStarted rt.Flag
	stop           yama.Stopper
	stopperStarted rt.Flag
}

func (lvl *mixedLevel) Start(ctx context.Context) error {
	g := rt.NewGroup(ctx)
	g.Go(lvl.start, &lvl.starterStarted)
	g.Go(rt.Started, &lvl.stopperStarted)
	return g.Wait()
}

func (lvl *mixedLevel) Stop(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.stop.Stop, &lvl.stopperStarted)
	g.Wait()
}

// stopOnlyLevel is a generated level whose single member implements only Stopper.
// Its start step is rt.Started, which marks the member reached; if the level is
// never reached (a later start failed first), the member is never marked and so
// is never torn down.
type stopOnlyLevel struct {
	stop    yama.Stopper
	started rt.Flag
}

func (lvl *stopOnlyLevel) Start(ctx context.Context) error {
	g := rt.NewGroup(ctx)
	g.Go(rt.Started, &lvl.started)
	return g.Wait()
}

func (lvl *stopOnlyLevel) Stop(ctx context.Context) {
	g := rt.NewShutdownGroup(ctx)
	g.Go(lvl.stop.Stop, &lvl.started)
	g.Wait()
}

// chainGraph models base → mid → top: three single-member levels in a dependency
// chain. Start runs 1→3; quiesce and teardown run 3→1.
type chainGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *soloLevel // base
	l2 *soloLevel // mid
	l3 *soloLevel // top
}

func newChain(chains *rt.Chains, begin, end *rt.Boundary, base, mid, top any) yama.Lifecycle {
	g := &chainGraph{
		begin: begin,
		end:   end,
		l1:    solo(chains, base),
		l2:    solo(chains, mid),
		l3:    solo(chains, top),
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *chainGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	if err := g.l2.Start(ctx); err != nil {
		return err
	}
	if err := g.l3.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *chainGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.l3.Quiesce(ctx)
	g.l2.Quiesce(ctx)
	g.l1.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l3.Stop(ctx)
	g.l2.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// diamondGraph models DB → {Router, Worker}: a single-member base level and a
// two-member level that runs concurrently.
type diamondGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *soloLevel // db
	l2 *pairLevel // router, worker
}

func newDiamond(chains *rt.Chains, begin, end *rt.Boundary, db, router, worker any) yama.Lifecycle {
	g := &diamondGraph{
		begin: begin,
		end:   end,
		l1:    solo(chains, db),
		l2:    pair(chains, router, worker),
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *diamondGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	if err := g.l2.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *diamondGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.l2.Quiesce(ctx)
	g.l1.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l2.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// transitiveGraph models base → mid → top where mid implements Starter and
// Stopper but not Quiescer. It proves quiesce ordering holds transitively through
// a non-Quiescer: mid's level joins the start and teardown passes but not quiesce.
type transitiveGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *soloLevel      // base
	l2 *startStopLevel // mid (non-quiescer)
	l3 *soloLevel      // top
}

func newTransitive(chains *rt.Chains, begin, end *rt.Boundary, base, mid, top any) yama.Lifecycle {
	g := &transitiveGraph{
		begin: begin,
		end:   end,
		l1:    solo(chains, base),
		l2:    startStop(chains, mid),
		l3:    solo(chains, top),
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *transitiveGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	if err := g.l2.Start(ctx); err != nil {
		return err
	}
	if err := g.l3.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *transitiveGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.l3.Quiesce(ctx)
	g.l1.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l3.Stop(ctx)
	g.l2.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// capabilityGraph holds one level with a Starter-only member and a Stopper-only
// member, proving a component receives only the callbacks for capabilities it
// implements.
type capabilityGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *mixedLevel
}

func newCapability(chains *rt.Chains, begin, end *rt.Boundary, starter, stopper any) yama.Lifecycle {
	g := &capabilityGraph{
		begin: begin,
		end:   end,
		l1: &mixedLevel{
			start: chains.WrapStart(starter),
			stop:  chains.WrapStop(stopper),
		},
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *capabilityGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *capabilityGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// singleGraph holds one full-capability component in a single level.
type singleGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *soloLevel
}

func newSingle(chains *rt.Chains, begin, end *rt.Boundary, only any) yama.Lifecycle {
	g := &singleGraph{
		begin: begin,
		end:   end,
		l1:    solo(chains, only),
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *singleGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *singleGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.l1.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// emptyGraph has no levels: only the boundary sets bracket its passes.
type emptyGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary
}

func newEmpty(begin, end *rt.Boundary) yama.Lifecycle {
	g := &emptyGraph{begin: begin, end: end}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *emptyGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *emptyGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.begin.Stop(ctx)
}

// reachedGraph models base → mid → late, where late implements only Stopper and
// sits in the deepest level. When mid's Start fails, late's level is never
// reached, so late must not be torn down during startup-failure cleanup.
type reachedGraph struct {
	begin *rt.Boundary
	end   *rt.Boundary

	l1 *soloLevel     // base (full)
	l2 *soloLevel     // mid (full; may fail)
	l3 *stopOnlyLevel // late (Stopper-only)
}

func newReached(chains *rt.Chains, begin, end *rt.Boundary, base, mid, late any) yama.Lifecycle {
	g := &reachedGraph{
		begin: begin,
		end:   end,
		l1:    solo(chains, base),
		l2:    solo(chains, mid),
		l3:    &stopOnlyLevel{stop: chains.WrapStop(late)},
	}
	return rt.NewLifecycle(g.startPass, g.shutdown)
}

func (g *reachedGraph) startPass(ctx context.Context) error {
	if err := g.begin.Start(ctx); err != nil {
		return err
	}
	if err := g.l1.Start(ctx); err != nil {
		return err
	}
	if err := g.l2.Start(ctx); err != nil {
		return err
	}
	if err := g.l3.Start(ctx); err != nil {
		return err
	}
	return g.end.Start(ctx)
}

func (g *reachedGraph) shutdown(ctx context.Context) {
	g.end.Quiesce(ctx)
	g.l2.Quiesce(ctx)
	g.l1.Quiesce(ctx)
	g.begin.Quiesce(ctx)

	g.end.Stop(ctx)
	g.l3.Stop(ctx)
	g.l2.Stop(ctx)
	g.l1.Stop(ctx)
	g.begin.Stop(ctx)
}

// firstIndexContaining returns the index of the first event containing sub, or -1.
func firstIndexContaining(events []string, sub string) int {
	for i, e := range events {
		if strings.Contains(e, sub) {
			return i
		}
	}
	return -1
}

// lastIndexContaining returns the index of the last event containing sub, or -1.
func lastIndexContaining(events []string, sub string) int {
	last := -1
	for i, e := range events {
		if strings.Contains(e, sub) {
			last = i
		}
	}
	return last
}
