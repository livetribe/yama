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
	"errors"
	"log/slog"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama"
	apimocks "l7e.io/yama/internal/mocks"
	execmocks "l7e.io/yama/rt/internal/mocks"
)

// The gates log these messages when they drop a component whose start failed.
const (
	chnQuiesceSkipMessage = "component startup failed, skipping quiesce"
	chnStopSkipMessage    = "component startup failed, skipping stop"
)

// chnBlocked is the event a non-delegating interceptor records.
const chnBlocked = "blocked"

// chnConcurrencyBound caps the wait for many concurrent wrappers to finish, and
// any spec that would otherwise hang rather than fail. It sits above goroutine
// scheduling latency and nothing more.
const chnConcurrencyBound = time.Second

// chnKey keys the context values a spec plants to follow a context down a chain.
type chnKey int

const (
	chnCallerKey chnKey = iota
	chnFirstKey
	chnSecondKey
)

// chnLog records chain events in the order they happened.
type chnLog struct {
	mu     sync.Mutex
	events []string
}

func (l *chnLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)
}

// seen returns a snapshot of everything recorded so far.
func (l *chnLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.events...)
}

// chnBarrier releases every arriving goroutine only once all of them have
// arrived, so a pass completes only when its members are genuinely in flight
// together.
type chnBarrier struct {
	wg sync.WaitGroup
}

func chnNewBarrier(members int) *chnBarrier {
	b := &chnBarrier{}
	b.wg.Add(members)

	return b
}

func (b *chnBarrier) arrive() {
	b.wg.Done()
	b.wg.Wait()
}

// chnBody is the behavior of a spec's interceptor: it receives the context that
// reached the interceptor and a func that runs the rest of the chain under a
// context of the body's choosing.
type chnBody func(ctx context.Context, next func(context.Context))

// chnEvery gives one value all three interceptor interfaces. Each embedded mock
// supplies its own operation's behavior.
type chnEvery struct {
	*apimocks.MockStartInterceptor
	*apimocks.MockQuiesceInterceptor
	*apimocks.MockStopInterceptor
}

// chnStarterStopper is a component that declares the start and stop capabilities
// and no other.
type chnStarterStopper struct {
	*apimocks.MockStarter
	*apimocks.MockStopper
}

// chnOp drives one lifecycle operation so a table can repeat a behavior across
// all three.
type chnOp struct {
	// name is the operation's name, and the event a component records when its
	// method for that operation runs.
	name string
	// build builds an interceptor for this operation whose behavior is body and
	// which must be invoked calls times.
	build func(ctrl *gomock.Controller, calls int, body chnBody) any
	// silent builds an interceptor for this operation with no expectation, so any
	// invocation of it fails the spec.
	silent func(*gomock.Controller) any
	// arrange makes the component's method for this operation run body.
	arrange func(*execmocks.MockCompleteLifecycle, func())
	// invoke runs this operation on a wrapped component.
	invoke func(context.Context, CompleteLifecycle)
}

// interceptor builds an interceptor for this operation whose behavior is body and
// which must be invoked exactly once.
func (op chnOp) interceptor(ctrl *gomock.Controller, body chnBody) any {
	return op.build(ctrl, 1, body)
}

// record builds an interceptor that logs name around its call to the rest of the
// chain and which must be invoked exactly once.
func (op chnOp) record(ctrl *gomock.Controller, log *chnLog, name string) any {
	return op.recordTimes(ctrl, log, name, 1)
}

// recordTimes builds an interceptor that logs name around its call to the rest of
// the chain and which must be invoked calls times.
func (op chnOp) recordTimes(ctrl *gomock.Controller, log *chnLog, name string, calls int) any {
	return op.build(ctrl, calls, func(ctx context.Context, next func(context.Context)) {
		log.add(name + " enter")
		next(ctx)
		log.add(name + " exit")
	})
}

// blocking builds an interceptor that logs chnBlocked and returns without running
// the rest of the chain.
func (op chnOp) blocking(ctrl *gomock.Controller, log *chnLog) any {
	return op.interceptor(ctrl, func(context.Context, func(context.Context)) {
		log.add(chnBlocked)
	})
}

// recordComponent makes the component log this operation when its method runs.
func (op chnOp) recordComponent(m *execmocks.MockCompleteLifecycle, log *chnLog) {
	op.arrange(m, func() {
		log.add(op.name)
	})
}

var chnStartOp = chnOp{
	name: "start",
	build: func(ctrl *gomock.Controller, calls int, body chnBody) any {
		m := apimocks.NewMockStartInterceptor(ctrl)
		m.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, next yama.Starter) error {
				var err error

				body(ctx, func(c context.Context) {
					err = next.Start(c)
				})

				return err
			}).Times(calls)

		return m
	},
	silent: func(ctrl *gomock.Controller) any {
		return apimocks.NewMockStartInterceptor(ctrl)
	},
	arrange: func(m *execmocks.MockCompleteLifecycle, body func()) {
		m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			body()

			return nil
		}).AnyTimes()
	},
	invoke: func(ctx context.Context, c CompleteLifecycle) {
		_ = c.Start(ctx)
	},
}

//nolint:dupl // mirrors chnStopOp; Quiesce and Stop share a shape but use distinct mock types.
var chnQuiesceOp = chnOp{
	name: "quiesce",
	build: func(ctrl *gomock.Controller, calls int, body chnBody) any {
		m := apimocks.NewMockQuiesceInterceptor(ctrl)
		m.EXPECT().Quiesce(gomock.Any(), gomock.Any()).Do(
			func(ctx context.Context, next yama.Quiescer) {
				body(ctx, next.Quiesce)
			}).Times(calls)

		return m
	},
	silent: func(ctrl *gomock.Controller) any {
		return apimocks.NewMockQuiesceInterceptor(ctrl)
	},
	arrange: func(m *execmocks.MockCompleteLifecycle, body func()) {
		m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
			body()
		}).AnyTimes()
	},
	invoke: func(ctx context.Context, c CompleteLifecycle) {
		c.Quiesce(ctx)
	},
}

//nolint:dupl // mirrors chnQuiesceOp; Quiesce and Stop share a shape but use distinct mock types.
var chnStopOp = chnOp{
	name: "stop",
	build: func(ctrl *gomock.Controller, calls int, body chnBody) any {
		m := apimocks.NewMockStopInterceptor(ctrl)
		m.EXPECT().Stop(gomock.Any(), gomock.Any()).Do(
			func(ctx context.Context, next yama.Stopper) {
				body(ctx, next.Stop)
			}).Times(calls)

		return m
	},
	silent: func(ctrl *gomock.Controller) any {
		return apimocks.NewMockStopInterceptor(ctrl)
	},
	arrange: func(m *execmocks.MockCompleteLifecycle, body func()) {
		m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
			body()
		}).AnyTimes()
	},
	invoke: func(ctx context.Context, c CompleteLifecycle) {
		c.Stop(ctx)
	},
}

// chnOps lists the three operations in the order a lifecycle runs them.
var chnOps = []chnOp{chnStartOp, chnQuiesceOp, chnStopOp}

// chnCalls is how many times a spec expects each operation's interceptor to run.
type chnCalls struct {
	start   int
	quiesce int
	stop    int
}

// chnEveryInterceptor builds one value that joins all three chains, logging name
// around its call to the rest of each chain.
func chnEveryInterceptor(ctrl *gomock.Controller, log *chnLog, name string) *chnEvery {
	return chnEveryFrom(
		chnStartOp.record(ctrl, log, name),
		chnQuiesceOp.record(ctrl, log, name),
		chnStopOp.record(ctrl, log, name),
	)
}

// chnEveryFrom joins one interceptor per operation into a single value that carries
// each one's expectation.
func chnEveryFrom(start, quiesce, stop any) *chnEvery {
	return &chnEvery{
		MockStartInterceptor:   start.(*apimocks.MockStartInterceptor),
		MockQuiesceInterceptor: quiesce.(*apimocks.MockQuiesceInterceptor),
		MockStopInterceptor:    stop.(*apimocks.MockStopInterceptor),
	}
}

// chnRecordAll makes a component log every operation it is asked to perform.
func chnRecordAll(m *execmocks.MockCompleteLifecycle, log *chnLog) {
	for _, op := range chnOps {
		op.recordComponent(m, log)
	}
}

// chnSilentAll returns one expectation-free interceptor per operation.
func chnSilentAll(ctrl *gomock.Controller) []any {
	registered := make([]any, 0, len(chnOps))
	for _, op := range chnOps {
		registered = append(registered, op.silent(ctrl))
	}

	return registered
}

// chnRunAll drives a wrapper through all three operations under ctx.
func chnRunAll(ctx context.Context, c CompleteLifecycle) {
	for _, op := range chnOps {
		op.invoke(ctx, c)
	}
}

// chnPanickingInterceptor registers an interceptor that panics instead of
// delegating.
func chnPanickingInterceptor(op chnOp, ctrl *gomock.Controller, _ *execmocks.MockCompleteLifecycle) []any {
	return []any{op.interceptor(ctrl, func(context.Context, func(context.Context)) {
		panic("boom")
	})}
}

// chnPanickingComponent registers a delegating interceptor over a component whose
// own method panics.
func chnPanickingComponent(op chnOp, ctrl *gomock.Controller, m *execmocks.MockCompleteLifecycle) []any {
	op.arrange(m, func() {
		panic("boom")
	})

	return []any{op.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
		next(ctx)
	})}
}

var _ = Describe("Chains interceptor registration", func() {
	var (
		ctrl *gomock.Controller
		log  *chnLog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = &chnLog{}
	})

	Context("membership", func() {
		DescribeTable("joins only the chain for the interface the interceptor implements",
			func(op chnOp, want []string) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				chnRunAll(context.Background(), NewChains([]any{op.record(ctrl, log, "only")}).WrapComponent(comp))

				Expect(log.seen()).To(Equal(want))
			},
			Entry("a start interceptor", chnStartOp,
				[]string{"only enter", "start", "only exit", "quiesce", "stop"}),
			Entry("a quiesce interceptor", chnQuiesceOp,
				[]string{"start", "only enter", "quiesce", "only exit", "stop"}),
			Entry("a stop interceptor", chnStopOp,
				[]string{"start", "quiesce", "only enter", "stop", "only exit"}),
		)

		It("joins every chain when one value implements all three interfaces", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			chnRecordAll(comp, log)

			chnRunAll(context.Background(), NewChains([]any{chnEveryInterceptor(ctrl, log, "all")}).WrapComponent(comp))

			Expect(log.seen()).To(Equal([]string{
				"all enter", "start", "all exit",
				"all enter", "quiesce", "all exit",
				"all enter", "stop", "all exit",
			}))
		})

		DescribeTable("accepts a registered value that implements no interceptor interface",
			func(build func(*gomock.Controller) any) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				var chains *Chains
				Expect(func() {
					chains = NewChains([]any{build(ctrl)})
				}).NotTo(Panic())

				chnRunAll(context.Background(), chains.WrapComponent(comp))

				Expect(log.seen()).To(Equal([]string{"start", "quiesce", "stop"}))
			},
			Entry("an untyped nil element", func(*gomock.Controller) any {
				return nil
			}),
			Entry("a value with no methods", func(*gomock.Controller) any {
				return struct{}{}
			}),
			Entry("a component-shaped value", func(ctrl *gomock.Controller) any {
				return execmocks.NewMockCompleteLifecycle(ctrl)
			}),
		)
	})

	Context("ordering", func() {
		DescribeTable("preserves registration order and reaches the component last",
			func(op chnOp) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				chains := NewChains([]any{
					op.record(ctrl, log, "first"),
					op.record(ctrl, log, "second"),
				})

				op.invoke(context.Background(), chains.WrapComponent(comp))

				Expect(log.seen()).To(Equal([]string{
					"first enter", "second enter", op.name, "second exit", "first exit",
				}))
			},
			Entry("start", chnStartOp),
			Entry("quiesce", chnQuiesceOp),
			Entry("stop", chnStopOp),
		)

		It("keeps each chain's relative order rather than regrouping by capability", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			chnRecordAll(comp, log)

			chains := NewChains([]any{
				chnStartOp.record(ctrl, log, "start-only"),
				chnEveryFrom(
					chnStartOp.record(ctrl, log, "both"),
					chnQuiesceOp.silent(ctrl),
					chnStopOp.record(ctrl, log, "both"),
				),
				chnStopOp.record(ctrl, log, "stop-only"),
			})

			wrapped := chains.WrapComponent(comp)
			Expect(wrapped.Start(context.Background())).To(Succeed())
			wrapped.Stop(context.Background())

			Expect(log.seen()).To(Equal([]string{
				"start-only enter", "both enter", "start", "both exit", "start-only exit",
				"both enter", "stop-only enter", "stop", "stop-only exit", "both exit",
			}))
		})

		It("runs one interceptor once per registration", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			chnRecordAll(comp, log)

			twice := chnStartOp.recordTimes(ctrl, log, "dup", 2)

			Expect(NewChains([]any{twice, twice}).WrapComponent(comp).Start(context.Background())).To(Succeed())

			Expect(log.seen()).To(Equal([]string{"dup enter", "dup enter", "start", "dup exit", "dup exit"}))
		})
	})

	Context("the built-in overrun interceptor", func() {
		DescribeTable("records nothing when a registered interceptor does not delegate",
			func(op chnOp) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)

				op.invoke(expired(), NewChains([]any{op.blocking(ctrl, log)}).WrapComponent(comp))

				Expect(log.seen()).To(Equal([]string{chnBlocked}))
				Expect(logs.records()).To(BeEmpty())
			},
			Entry("start", chnStartOp),
			Entry("quiesce", chnQuiesceOp),
			Entry("stop", chnStopOp),
		)

		DescribeTable("has already recorded the overrun when a registered interceptor regains control",
			func(op chnOp) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				seen := -1
				outer := op.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
					next(ctx)
					seen = len(logs.records())
				})

				op.invoke(expired(), NewChains([]any{outer}).WrapComponent(comp))

				Expect(seen).To(Equal(1))
				Expect(logs.records()).To(HaveLen(1))
				Expect(logs.records()[0].Message).To(Equal(overrunMessage))
			},
			Entry("start", chnStartOp),
			Entry("quiesce", chnQuiesceOp),
			Entry("stop", chnStopOp),
		)

		It("records once however many interceptors and components share the chains", func() {
			logs := captureSlog()

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			chnRecordAll(comp, log)

			chains := NewChains([]any{
				chnStartOp.record(ctrl, log, "a"),
				chnStartOp.record(ctrl, log, "b"),
				chnStartOp.record(ctrl, log, "c"),
			})

			chains.WrapComponent(execmocks.NewMockCompleteLifecycle(ctrl))
			Expect(chains.WrapComponent(comp).Start(expired())).To(Succeed())

			Expect(log.seen()).To(Equal([]string{
				"a enter", "b enter", "c enter", "start", "c exit", "b exit", "a exit",
			}))
			Expect(logs.records()).To(HaveLen(1))
			Expect(logs.records()[0].Message).To(Equal(overrunMessage))
		})

		DescribeTable("observes the context a registered interceptor substituted",
			func(op chnOp) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				undated := op.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
					next(context.WithoutCancel(ctx))
				})

				op.invoke(expired(), NewChains([]any{undated}).WrapComponent(comp))

				Expect(log.seen()).To(Equal([]string{op.name}))
				Expect(logs.records()).To(BeEmpty())
			},
			Entry("start", chnStartOp),
			Entry("quiesce", chnQuiesceOp),
			Entry("stop", chnStopOp),
		)

		DescribeTable("sits directly above the component when nothing is registered",
			func(interceptors []any) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				Expect(NewChains(interceptors).WrapComponent(comp).Start(expired())).To(Succeed())

				Expect(log.seen()).To(Equal([]string{"start"}))
				Expect(logs.records()).To(HaveLen(1))
				Expect(logs.records()[0].Message).To(Equal(overrunMessage))
			},
			Entry("a nil interceptor slice", []any(nil)),
			Entry("an empty interceptor slice", []any{}),
		)
	})

	Context("the quiesce and stop gates", func() {
		DescribeTable("keep a component whose start failed out of the whole chain",
			func(op chnOp, message string) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

				wrapped := NewChains([]any{op.silent(ctrl)}).WrapComponent(comp)
				Expect(wrapped.Start(context.Background())).NotTo(Succeed())

				op.invoke(expired(), wrapped)

				recs := logs.records()
				Expect(recs).To(HaveLen(1))
				Expect(recs[0].Level).To(Equal(slog.LevelWarn))
				Expect(recs[0].Message).To(Equal(message))
			},
			Entry("quiesce", chnQuiesceOp, chnQuiesceSkipMessage),
			Entry("stop", chnStopOp, chnStopSkipMessage),
		)

		DescribeTable("name the component they skipped",
			func(op chnOp, build func(*execmocks.MockCompleteLifecycle) any, want string) {
				logs := captureSlog()

				m := execmocks.NewMockCompleteLifecycle(ctrl)
				m.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

				wrapped := NewChains(nil).WrapComponent(build(m))
				Expect(wrapped.Start(context.Background())).NotTo(Succeed())

				op.invoke(context.Background(), wrapped)

				recs := logs.records()
				Expect(recs).To(HaveLen(1))
				Expect(attrs(recs[0])["component"].String()).To(Equal(want))
			},
			Entry("quiesce, a Stringer component", chnQuiesceOp,
				func(m *execmocks.MockCompleteLifecycle) any {
					return &namedComponent{MockCompleteLifecycle: m, name: "kafka-consumer"}
				}, "kafka-consumer"),
			Entry("quiesce, a component that is not a Stringer", chnQuiesceOp,
				func(m *execmocks.MockCompleteLifecycle) any {
					return m
				}, "*mocks.MockCompleteLifecycle"),
			Entry("stop, a Stringer component", chnStopOp,
				func(m *execmocks.MockCompleteLifecycle) any {
					return &namedComponent{MockCompleteLifecycle: m, name: "kafka-consumer"}
				}, "kafka-consumer"),
			Entry("stop, a component that is not a Stringer", chnStopOp,
				func(m *execmocks.MockCompleteLifecycle) any {
					return m
				}, "*mocks.MockCompleteLifecycle"),
		)

		DescribeTable("let a component that was never started through",
			func(op chnOp) {
				logs := captureSlog()

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				chnRecordAll(comp, log)

				op.invoke(context.Background(), NewChains([]any{op.record(ctrl, log, "i")}).WrapComponent(comp))

				Expect(log.seen()).To(Equal([]string{"i enter", op.name, "i exit"}))
				Expect(logs.records()).To(BeEmpty())
			},
			Entry("quiesce", chnQuiesceOp),
			Entry("stop", chnStopOp),
		)

		It("runs the whole quiesce chain when no component status is attached", func() {
			comp := apimocks.NewMockQuiescer(ctrl)
			comp.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
				log.add("quiesce")
			})

			chains := NewChains([]any{chnQuiesceOp.record(ctrl, log, "i")})
			chains.wrapQuiesce(comp).Quiesce(context.Background())

			Expect(log.seen()).To(Equal([]string{"i enter", "quiesce", "i exit"}))
		})

		It("runs the whole stop chain when no component status is attached", func() {
			comp := apimocks.NewMockStopper(ctrl)
			comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				log.add("stop")
			})

			chains := NewChains([]any{chnStopOp.record(ctrl, log, "i")})
			chains.wrapStop(comp).Stop(context.Background())

			Expect(log.seen()).To(Equal([]string{"i enter", "stop", "i exit"}))
		})

		It("guards no start chain, so a start that failed runs again in full", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				log.add("start")

				return errors.New("boom")
			})
			comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				log.add("start")

				return nil
			})

			wrapped := NewChains([]any{chnStartOp.recordTimes(ctrl, log, "i", 2)}).WrapComponent(comp)
			Expect(wrapped.Start(context.Background())).NotTo(Succeed())
			Expect(wrapped.Start(context.Background())).To(Succeed())

			Expect(log.seen()).To(Equal([]string{
				"i enter", "start", "i exit", "i enter", "start", "i exit",
			}))
		})
	})
})

var _ = Describe("Chains component wrapping", func() {
	var (
		ctrl *gomock.Controller
		log  *chnLog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = &chnLog{}
	})

	It("invokes nothing at all for a capability the component lacks", func() {
		logs := captureSlog()

		comp := apimocks.NewMockStarter(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			log.add("start")

			return nil
		})

		chains := NewChains([]any{
			chnStartOp.record(ctrl, log, "start-i"),
			chnQuiesceOp.silent(ctrl),
			chnStopOp.silent(ctrl),
		})

		chnRunAll(expired(), chains.WrapComponent(comp))

		Expect(log.seen()).To(Equal([]string{"start-i enter", "start", "start-i exit"}))

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Message).To(Equal(overrunMessage))
	})

	DescribeTable("participates in exactly the passes the component declared",
		func(build func(*gomock.Controller, *chnLog) any, calls chnCalls, want []string) {
			chains := NewChains([]any{
				chnStartOp.recordTimes(ctrl, log, "start-i", calls.start),
				chnQuiesceOp.recordTimes(ctrl, log, "quiesce-i", calls.quiesce),
				chnStopOp.recordTimes(ctrl, log, "stop-i", calls.stop),
			})

			chnRunAll(context.Background(), chains.WrapComponent(build(ctrl, log)))

			Expect(log.seen()).To(Equal(want))
		},
		Entry("a Starter", func(ctrl *gomock.Controller, log *chnLog) any {
			m := apimocks.NewMockStarter(ctrl)
			m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				log.add("start")

				return nil
			})

			return m
		}, chnCalls{start: 1}, []string{"start-i enter", "start", "start-i exit"}),
		Entry("a Quiescer", func(ctrl *gomock.Controller, log *chnLog) any {
			m := apimocks.NewMockQuiescer(ctrl)
			m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
				log.add("quiesce")
			})

			return m
		}, chnCalls{quiesce: 1}, []string{"quiesce-i enter", "quiesce", "quiesce-i exit"}),
		Entry("a Stopper", func(ctrl *gomock.Controller, log *chnLog) any {
			m := apimocks.NewMockStopper(ctrl)
			m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				log.add("stop")
			})

			return m
		}, chnCalls{stop: 1}, []string{"stop-i enter", "stop", "stop-i exit"}),
		Entry("a Starter that is also a Stopper", func(ctrl *gomock.Controller, log *chnLog) any {
			c := &chnStarterStopper{
				MockStarter: apimocks.NewMockStarter(ctrl),
				MockStopper: apimocks.NewMockStopper(ctrl),
			}
			c.MockStarter.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				log.add("start")

				return nil
			})
			c.MockStopper.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				log.add("stop")
			})

			return c
		}, chnCalls{start: 1, stop: 1},
			[]string{"start-i enter", "start", "start-i exit", "stop-i enter", "stop", "stop-i exit"}),
	)

	DescribeTable("is inert when the value declares no capability",
		func(value any) {
			logs := captureSlog()

			wrapped := NewChains(chnSilentAll(ctrl)).WrapComponent(value)

			bound, ok := wrapped.(*component)
			Expect(ok).To(BeTrue())
			Expect(bound.start).To(BeNil())
			Expect(bound.quiesce).To(BeNil())
			Expect(bound.stop).To(BeNil())

			Expect(wrapped.Start(expired())).To(Succeed())
			wrapped.Quiesce(expired())
			wrapped.Stop(expired())

			Expect(logs.records()).To(BeEmpty())
		},
		Entry("a nil value", nil),
		Entry("a value with no lifecycle methods", struct{}{}),
	)

	DescribeTable("hands the original component to the first interceptor in the chain",
		func(op chnOp) {
			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "gateway",
			}
			chnRecordAll(comp.MockCompleteLifecycle, log)

			var (
				got *namedComponent
				ok  bool
			)

			outer := op.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
				got, ok = yama.FromContext[*namedComponent](ctx)
				next(ctx)
			})

			op.invoke(context.Background(), NewChains([]any{outer}).WrapComponent(comp))

			Expect(ok).To(BeTrue())
			Expect(got).To(BeIdenticalTo(comp))
			Expect(log.seen()).To(Equal([]string{op.name}))
		},
		Entry("start", chnStartOp),
		Entry("quiesce", chnQuiesceOp),
		Entry("stop", chnStopOp),
	)

	It("carries the caller's context down the chain and each substitution with it", func() {
		var (
			seen        []any
			hasDeadline bool
		)

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			seen = []any{ctx.Value(chnCallerKey), ctx.Value(chnFirstKey), ctx.Value(chnSecondKey)}
			_, hasDeadline = ctx.Deadline()

			return nil
		})

		first := chnStartOp.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
			next(context.WithValue(ctx, chnFirstKey, "first"))
		})
		second := chnStartOp.interceptor(ctrl, func(ctx context.Context, next func(context.Context)) {
			Expect(ctx.Value(chnFirstKey)).To(Equal("first"))
			next(context.WithValue(ctx, chnSecondKey, "second"))
		})

		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), chnCallerKey, "caller"), time.Hour)
		DeferCleanup(cancel)

		Expect(NewChains([]any{first, second}).WrapComponent(comp).Start(ctx)).To(Succeed())

		Expect(seen).To(Equal([]any{"caller", "first", "second"}))
		Expect(hasDeadline).To(BeTrue())
	})
})

var _ = Describe("Chains reuse and isolation", func() {
	var (
		ctrl *gomock.Controller
		log  *chnLog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = &chnLog{}
	})

	It("presents the identical downstream chain on every invocation of a wrapper", func() {
		var seen []yama.Starter

		observer := apimocks.NewMockStartInterceptor(ctrl)
		observer.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, next yama.Starter) error {
				seen = append(seen, next)

				return next.Start(ctx)
			}).Times(3)

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Times(3)

		wrapped := NewChains([]any{observer}).WrapComponent(comp)
		for range 3 {
			Expect(wrapped.Start(context.Background())).To(Succeed())
		}

		Expect(seen).To(HaveLen(3))
		Expect(seen[1]).To(BeIdenticalTo(seen[0]))
		Expect(seen[2]).To(BeIdenticalTo(seen[0]))
	})

	It("gives each wrapped component its own chain instances", func() {
		var seen []yama.Starter

		observer := apimocks.NewMockStartInterceptor(ctrl)
		observer.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, next yama.Starter) error {
				seen = append(seen, next)

				return next.Start(ctx)
			}).Times(2)

		first := execmocks.NewMockCompleteLifecycle(ctrl)
		first.EXPECT().Start(gomock.Any())
		second := execmocks.NewMockCompleteLifecycle(ctrl)
		second.EXPECT().Start(gomock.Any())

		chains := NewChains([]any{observer})
		Expect(chains.WrapComponent(first).Start(context.Background())).To(Succeed())
		Expect(chains.WrapComponent(second).Start(context.Background())).To(Succeed())

		Expect(seen).To(HaveLen(2))
		Expect(seen[1]).NotTo(BeIdenticalTo(seen[0]))
	})

	It("does not grow the chains as more components are wrapped", func() {
		logs := captureSlog()

		chains := NewChains([]any{
			chnStartOp.record(ctrl, log, "a"),
			chnStartOp.record(ctrl, log, "b"),
			chnQuiesceOp.silent(ctrl),
		})

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

		var last CompleteLifecycle
		for range 4 {
			last = chains.WrapComponent(comp)
		}

		Expect(last.Start(expired())).NotTo(Succeed())
		last.Quiesce(context.Background())

		Expect(log.seen()).To(Equal([]string{"a enter", "b enter", "b exit", "a exit"}))

		recs := logs.records()
		Expect(recs).To(HaveLen(2))
		Expect(recs[0].Message).To(Equal(overrunMessage))
		Expect(recs[1].Message).To(Equal(chnQuiesceSkipMessage))
	})

	It("ignores mutation of the caller's interceptor slice", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		chnRecordAll(comp, log)

		registered := []any{chnStartOp.record(ctrl, log, "kept")}
		chains := NewChains(registered)
		registered[0] = chnStartOp.silent(ctrl)

		Expect(chains.WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(log.seen()).To(Equal([]string{"kept enter", "start", "kept exit"}))
	})

	It("keeps one component's failed start off another wrapped from the same chains", func() {
		logs := captureSlog()

		failing := execmocks.NewMockCompleteLifecycle(ctrl)
		failing.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

		healthy := execmocks.NewMockCompleteLifecycle(ctrl)
		chnRecordAll(healthy, log)

		chains := NewChains(nil)
		brokenWrapper := chains.WrapComponent(failing)
		healthyWrapper := chains.WrapComponent(healthy)

		Expect(healthyWrapper.Start(context.Background())).To(Succeed())
		Expect(brokenWrapper.Start(context.Background())).NotTo(Succeed())

		healthyWrapper.Quiesce(context.Background())
		healthyWrapper.Stop(context.Background())

		Expect(log.seen()).To(Equal([]string{"start", "quiesce", "stop"}))
		Expect(logs.records()).To(BeEmpty())
	})

	It("wraps one value twice into wrappers with independent start state", func() {
		logs := captureSlog()

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))
		comp.EXPECT().Quiesce(gomock.Any())

		chains := NewChains(nil)
		started := chains.WrapComponent(comp)
		untouched := chains.WrapComponent(comp)

		Expect(started.Start(context.Background())).NotTo(Succeed())

		untouched.Quiesce(context.Background())
		started.Quiesce(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Message).To(Equal(chnQuiesceSkipMessage))
	})

	It("drives many wrappers built from one Chains concurrently", func() {
		const members = 16

		barriers := map[string]*chnBarrier{}
		for _, op := range chnOps {
			barriers[op.name] = chnNewBarrier(members)
		}

		chains := NewChains([]any{
			chnStartOp.recordTimes(ctrl, log, "start-i", members),
			chnQuiesceOp.recordTimes(ctrl, log, "quiesce-i", members),
			chnStopOp.recordTimes(ctrl, log, "stop-i", members),
		})

		var wg sync.WaitGroup

		for range members {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			for _, op := range chnOps {
				op.arrange(comp, barriers[op.name].arrive)
			}

			wrapped := chains.WrapComponent(comp)

			wg.Add(1)

			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				Expect(wrapped.Start(context.Background())).To(Succeed())
				wrapped.Quiesce(context.Background())
				wrapped.Stop(context.Background())
			}()
		}

		done := make(chan struct{})

		go func() {
			wg.Wait()
			close(done)
		}()

		Eventually(done, chnConcurrencyBound).Should(BeClosed())
		Expect(log.seen()).To(HaveLen(members * 6))
	})
})

var _ = Describe("Chains outcomes", func() {
	var (
		ctrl *gomock.Controller
		log  *chnLog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = &chnLog{}
	})

	It("returns the component's own error rather than the lifecycle error", func() {
		boom := errors.New("boom")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(boom)

		err := NewChains([]any{chnStartOp.record(ctrl, log, "i")}).WrapComponent(comp).Start(context.Background())

		Expect(err).To(MatchError(boom))
		Expect(errors.Is(err, yama.ErrStartFailed)).To(BeFalse())
		Expect(log.seen()).To(Equal([]string{"i enter", "i exit"}))
	})

	//nolint:dupl // mirrors interceptor_test.go's single-interceptor case; different subject under test.
	It("lets a start interceptor turn a failed component into a started one", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))
		comp.EXPECT().Quiesce(gomock.Any())
		comp.EXPECT().Stop(gomock.Any())

		forgiving := apimocks.NewMockStartInterceptor(ctrl)
		forgiving.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, next yama.Starter) error {
				_ = next.Start(ctx)

				return nil
			})

		wrapped := NewChains([]any{forgiving}).WrapComponent(comp)
		Expect(wrapped.Start(context.Background())).To(Succeed())

		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())
	})

	It("suppresses the component's start when a start interceptor fails without delegating", func() {
		logs := captureSlog()

		boom := errors.New("boom")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)

		refusing := apimocks.NewMockStartInterceptor(ctrl)
		refusing.EXPECT().Start(gomock.Any(), gomock.Any()).Return(boom)

		wrapped := NewChains([]any{refusing}).WrapComponent(comp)
		Expect(wrapped.Start(context.Background())).To(MatchError(boom))

		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))
		Expect(recs[0].Message).To(Equal(chnQuiesceSkipMessage))
		Expect(recs[1].Message).To(Equal(chnStopSkipMessage))
	})

	DescribeTable("propagates a panic raised inside the chain",
		func(op chnOp, arrange func(chnOp, *gomock.Controller, *execmocks.MockCompleteLifecycle) []any) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			wrapped := NewChains(arrange(op, ctrl, comp)).WrapComponent(comp)

			Expect(func() {
				op.invoke(context.Background(), wrapped)
			}).To(PanicWith("boom"))
		},
		Entry("start, from an interceptor", chnStartOp, chnPanickingInterceptor),
		Entry("start, from the component", chnStartOp, chnPanickingComponent),
		Entry("quiesce, from an interceptor", chnQuiesceOp, chnPanickingInterceptor),
		Entry("quiesce, from the component", chnQuiesceOp, chnPanickingComponent),
		Entry("stop, from an interceptor", chnStopOp, chnPanickingInterceptor),
		Entry("stop, from the component", chnStopOp, chnPanickingComponent),
	)
})
