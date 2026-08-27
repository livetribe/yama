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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama"
	apimocks "l7e.io/yama/internal/mocks"
	execmocks "l7e.io/yama/rt/internal/mocks"
)

const (
	cmpQuiesceSkipMessage = "component startup failed, skipping quiesce"
	cmpStopSkipMessage    = "component startup failed, skipping stop"
)

var errCmpBoom = errors.New("component start failed")

// cmpNoStarter participates in quiesce and teardown but not startup. The
// embedded mocks carry the behavior and promote Quiesce and Stop; nothing here
// implements Start.
type cmpNoStarter struct {
	*apimocks.MockQuiescer
	*apimocks.MockStopper
}

// cmpValueKey keys a value placed on the caller's context.
type cmpValueKey struct{}

var _ = Describe("Component wrapper capability binding", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	Context("when the component implements no Starter", func() {
		It("starts silently and still runs both shutdown passes", func() {
			comp := &cmpNoStarter{
				MockQuiescer: apimocks.NewMockQuiescer(ctrl),
				MockStopper:  apimocks.NewMockStopper(ctrl),
			}
			comp.MockQuiescer.EXPECT().Quiesce(gomock.Any())
			comp.MockStopper.EXPECT().Stop(gomock.Any())

			wrapped := NewChains(nil).WrapComponent(comp)

			Expect(wrapped.Start(context.Background())).To(Succeed())
			wrapped.Quiesce(context.Background())
			wrapped.Stop(context.Background())

			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("when the component implements no Quiescer", func() {
		It("quiesces as a no-op and still tears the component down", func() {
			comp := apimocks.NewMockStopper(ctrl)
			comp.EXPECT().Stop(gomock.Any())

			wrapped := NewChains(nil).WrapComponent(comp)

			Expect(func() { wrapped.Quiesce(context.Background()) }).NotTo(Panic())
			wrapped.Stop(context.Background())
		})
	})

	Context("when the component implements no Stopper", func() {
		It("tears down as a no-op and still quiesces the component", func() {
			comp := apimocks.NewMockQuiescer(ctrl)
			comp.EXPECT().Quiesce(gomock.Any())

			wrapped := NewChains(nil).WrapComponent(comp)

			wrapped.Quiesce(context.Background())
			Expect(func() { wrapped.Stop(context.Background()) }).NotTo(Panic())
		})
	})

	Context("when the component implements none of the capabilities", func() {
		It("is inert in all three passes", func() {
			wrapped := NewChains(nil).WrapComponent(&inertComponent{})

			Expect(wrapped.Start(context.Background())).To(Succeed())
			Expect(func() { wrapped.Quiesce(context.Background()) }).NotTo(Panic())
			Expect(func() { wrapped.Stop(context.Background()) }).NotTo(Panic())

			Expect(logs.records()).To(BeEmpty())
		})
	})
})

var _ = Describe("Component wrapper start outcome", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	It("calls the component once and returns its own error unchanged", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom).Times(1)

		err := NewChains(nil).WrapComponent(comp).Start(context.Background())

		Expect(err).To(MatchError(errCmpBoom))
		Expect(err).NotTo(MatchError(yama.ErrStartFailed))
	})

	It("keeps a component whose Start failed out of both shutdown passes", func() {
		quiesceInterceptor := apimocks.NewMockQuiesceInterceptor(ctrl)
		stopInterceptor := apimocks.NewMockStopInterceptor(ctrl)

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains([]any{quiesceInterceptor, stopInterceptor}).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		Expect(logs.records()).To(HaveLen(2))
	})

	It("propagates a panicking Start unchanged", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			panic(errCmpBoom)
		})

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(func() { _ = wrapped.Start(context.Background()) }).To(PanicWith(errCmpBoom))
	})

	It("keeps a component whose Start panicked out of both shutdown passes", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			panic(errCmpBoom)
		})

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(func() { _ = wrapped.Start(context.Background()) }).To(PanicWith(errCmpBoom))
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))
		Expect(recs[0].Message).To(Equal(cmpQuiesceSkipMessage))
		Expect(recs[1].Message).To(Equal(cmpStopSkipMessage))
	})

	It("stops skipping a component that started cleanly on a later attempt", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)
		comp.EXPECT().Start(gomock.Any())
		comp.EXPECT().Quiesce(gomock.Any())
		comp.EXPECT().Stop(gomock.Any())

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		Expect(wrapped.Start(context.Background())).To(Succeed())
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		Expect(logs.records()).To(BeEmpty())
	})

	It("skips a component that started cleanly and then failed a later attempt", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any())
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(Succeed())
		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		Expect(logs.records()).To(HaveLen(2))
	})

	It("skips only the sibling whose own Start failed", func() {
		failing := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "failing",
		}
		failing.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		clean := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "clean",
		}
		clean.EXPECT().Start(gomock.Any())
		clean.EXPECT().Quiesce(gomock.Any())
		clean.EXPECT().Stop(gomock.Any())

		chains := NewChains(nil)
		wrappedFailing, wrappedClean := chains.WrapComponent(failing), chains.WrapComponent(clean)

		Expect(wrappedFailing.Start(context.Background())).To(MatchError(errCmpBoom))
		Expect(wrappedClean.Start(context.Background())).To(Succeed())

		wrappedFailing.Quiesce(context.Background())
		wrappedClean.Quiesce(context.Background())
		wrappedFailing.Stop(context.Background())
		wrappedClean.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))
		Expect(attrs(recs[0])["component"].String()).To(Equal("failing"))
		Expect(attrs(recs[1])["component"].String()).To(Equal("failing"))
	})

	DescribeTable("propagates a panicking shutdown pass unchanged",
		func(arrange func(*execmocks.MockCompleteLifecycle), invoke func(CompleteLifecycle, context.Context)) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp)

			wrapped := NewChains(nil).WrapComponent(comp)

			Expect(func() { invoke(wrapped, context.Background()) }).To(PanicWith(errCmpBoom))
			Expect(logs.records()).To(BeEmpty())
		},
		Entry("quiesce",
			func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
					panic(errCmpBoom)
				})
			},
			func(c CompleteLifecycle, ctx context.Context) {
				c.Quiesce(ctx)
			}),
		Entry("stop",
			func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
					panic(errCmpBoom)
				})
			},
			func(c CompleteLifecycle, ctx context.Context) {
				c.Stop(ctx)
			}),
	)

	It("treats a component that was never started as started", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Quiesce(gomock.Any())
		comp.EXPECT().Stop(gomock.Any())

		wrapped := NewChains(nil).WrapComponent(comp)

		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		Expect(logs.records()).To(BeEmpty())
	})
})

var _ = Describe("Component status context", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	DescribeTable("carries the outcome of Start into the pass",
		func(startErr error, want bool,
			arrange func(*execmocks.MockCompleteLifecycle, func(context.Context)),
			invoke func(*component, context.Context)) {
			var status, ok bool
			record := func(ctx context.Context) {
				status, ok = statusFromContext(ctx)
			}

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(startErr)
			arrange(comp, record)

			c := &component{start: comp, quiesce: comp, stop: comp}
			_ = c.Start(context.Background())
			invoke(c, context.Background())

			Expect(ok).To(BeTrue())
			Expect(status).To(Equal(want))
		},
		Entry("quiesce after a clean start", nil, false,
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Quiesce(gomock.Any()).Do(record)
			},
			func(c *component, ctx context.Context) {
				c.Quiesce(ctx)
			}),
		Entry("quiesce after a failed start", errCmpBoom, true,
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Quiesce(gomock.Any()).Do(record)
			},
			func(c *component, ctx context.Context) {
				c.Quiesce(ctx)
			}),
		Entry("stop after a clean start", nil, false,
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Stop(gomock.Any()).Do(record)
			},
			func(c *component, ctx context.Context) {
				c.Stop(ctx)
			}),
		Entry("stop after a failed start", errCmpBoom, true,
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Stop(gomock.Any()).Do(record)
			},
			func(c *component, ctx context.Context) {
				c.Stop(ctx)
			}),
	)

	DescribeTable("reports the most recent Start, not the first",
		func(outcomes []error, want bool) {
			var status, ok bool

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			for _, err := range outcomes {
				comp.EXPECT().Start(gomock.Any()).Return(err)
			}
			comp.EXPECT().Quiesce(gomock.Any()).Do(func(ctx context.Context) {
				status, ok = statusFromContext(ctx)
			})

			c := &component{start: comp, quiesce: comp}
			for range outcomes {
				_ = c.Start(context.Background())
			}
			c.Quiesce(context.Background())

			Expect(ok).To(BeTrue())
			Expect(status).To(Equal(want))
		},
		Entry("never started", nil, false),
		Entry("failed then clean", []error{errCmpBoom, nil}, false),
		Entry("clean then failed", []error{nil, errCmpBoom}, true),
	)

	It("carries the outcome into Stop whether or not Quiesce ran first", func() {
		var status, ok bool

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)
		comp.EXPECT().Quiesce(gomock.Any())
		comp.EXPECT().Stop(gomock.Any()).Do(func(ctx context.Context) {
			status, ok = statusFromContext(ctx)
		})

		c := &component{start: comp, quiesce: comp, stop: comp}

		_ = c.Start(context.Background())
		c.Quiesce(context.Background())
		c.Stop(context.Background())

		Expect(ok).To(BeTrue())
		Expect(status).To(BeTrue())
	})

	DescribeTable("preserves the caller's values and cancellation",
		func(arrange func(*execmocks.MockCompleteLifecycle, func(context.Context)), invoke func(CompleteLifecycle, context.Context)) {
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), cmpValueKey{}, "tenant-7"))
			cancel()

			var seen context.Context
			record := func(passed context.Context) {
				seen = passed
			}

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp, record)

			invoke(NewChains(nil).WrapComponent(comp), ctx)

			Expect(seen).NotTo(BeNil())
			Expect(seen.Value(cmpValueKey{})).To(Equal("tenant-7"))
			Expect(seen.Err()).To(MatchError(context.Canceled))
		},
		Entry("into Start",
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Start(gomock.Any()).Do(record)
			},
			func(c CompleteLifecycle, ctx context.Context) {
				_ = c.Start(ctx)
			}),
		Entry("into Quiesce",
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Quiesce(gomock.Any()).Do(record)
			},
			func(c CompleteLifecycle, ctx context.Context) {
				c.Quiesce(ctx)
			}),
		Entry("into Stop",
			func(m *execmocks.MockCompleteLifecycle, record func(context.Context)) {
				m.EXPECT().Stop(gomock.Any()).Do(record)
			},
			func(c CompleteLifecycle, ctx context.Context) {
				c.Stop(ctx)
			}),
	)

	It("preserves the caller's deadline, so a clean component still overruns", func() {
		logs := captureSlog()

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any())
		comp.EXPECT().Quiesce(gomock.Any())

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(Succeed())
		wrapped.Quiesce(expired())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Message).To(Equal(overrunMessage))
	})

	It("reports no status for a context that carries none", func() {
		status, ok := statusFromContext(context.Background())

		Expect(ok).To(BeFalse())
		Expect(status).To(BeFalse())
	})

	DescribeTable("round-trips what withStatus wrote",
		func(failed bool) {
			status, ok := statusFromContext(withStatus(context.Background(), failed))

			Expect(ok).To(BeTrue())
			Expect(status).To(Equal(failed))
		},
		Entry("a failed start", true),
		Entry("a clean start", false),
	)

	It("reports no status when the key carries a non-bool", func() {
		status, ok := statusFromContext(context.WithValue(context.Background(), statusKey{}, "failed"))

		Expect(ok).To(BeFalse())
		Expect(status).To(BeFalse())
	})
})

var _ = Describe("Startup-failure gates", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	DescribeTable("let the quiesce through",
		func(ctx context.Context) {
			next := apimocks.NewMockQuiescer(ctrl)
			next.EXPECT().Quiesce(ctx)

			QuiesceGate.Quiesce(ctx, next)

			Expect(logs.records()).To(BeEmpty())
		},
		Entry("with no status", context.Background()),
		Entry("with a clean status", withStatus(context.Background(), false)),
		Entry("with a non-bool status", context.WithValue(context.Background(), statusKey{}, "failed")),
	)

	DescribeTable("let the teardown through",
		func(ctx context.Context) {
			next := apimocks.NewMockStopper(ctrl)
			next.EXPECT().Stop(ctx)

			StopGate.Stop(ctx, next)

			Expect(logs.records()).To(BeEmpty())
		},
		Entry("with no status", context.Background()),
		Entry("with a clean status", withStatus(context.Background(), false)),
		Entry("with a non-bool status", context.WithValue(context.Background(), statusKey{}, "failed")),
	)

	//nolint:dupl // mirrors the Stop case below; QuiesceGate and StopGate are distinct gates.
	It("drop the quiesce of a failed component without panicking", func() {
		next := apimocks.NewMockQuiescer(ctrl)

		Expect(func() {
			QuiesceGate.Quiesce(withStatus(context.Background(), true), next)
		}).NotTo(Panic())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Level).To(Equal(slog.LevelWarn))
		Expect(recs[0].Message).To(Equal(cmpQuiesceSkipMessage))
		Expect(attrs(recs[0])["component"].String()).To(Equal("unknown"))
	})

	It("log the skipped quiesce under the caller's context", func() {
		ctx := withStatus(context.WithValue(context.Background(), cmpValueKey{}, "tenant-7"), true)

		QuiesceGate.Quiesce(ctx, apimocks.NewMockQuiescer(ctrl))

		ctxs := logs.contexts()
		Expect(ctxs).To(HaveLen(1))
		Expect(ctxs[0].Value(cmpValueKey{})).To(Equal("tenant-7"))
	})

	//nolint:dupl // mirrors the Quiesce case above; QuiesceGate and StopGate are distinct gates.
	It("drop the teardown of a failed component without panicking", func() {
		next := apimocks.NewMockStopper(ctrl)

		Expect(func() {
			StopGate.Stop(withStatus(context.Background(), true), next)
		}).NotTo(Panic())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Level).To(Equal(slog.LevelWarn))
		Expect(recs[0].Message).To(Equal(cmpStopSkipMessage))
		Expect(attrs(recs[0])["component"].String()).To(Equal("unknown"))
	})

	It("log the skipped teardown under the caller's context", func() {
		ctx := withStatus(context.WithValue(context.Background(), cmpValueKey{}, "tenant-7"), true)

		StopGate.Stop(ctx, apimocks.NewMockStopper(ctrl))

		ctxs := logs.contexts()
		Expect(ctxs).To(HaveLen(1))
		Expect(ctxs[0].Value(cmpValueKey{})).To(Equal("tenant-7"))
	})

	It("name a skipped Stringer component by its String", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "kafka-consumer",
		}
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Quiesce(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(attrs(recs[0])["component"].String()).To(Equal("kafka-consumer"))
	})

	It("name a skipped component that is not a Stringer by its type", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(attrs(recs[0])["component"].String()).To(Equal("*mocks.MockCompleteLifecycle"))
	})

	DescribeTable("keep a skipped component away from the built-in overrun interceptor",
		func(invoke func(CompleteLifecycle, context.Context), want string) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

			wrapped := NewChains(nil).WrapComponent(comp)

			Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
			invoke(wrapped, expired())

			recs := logs.records()
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Message).To(Equal(want))
		},
		Entry("quiesce",
			func(c CompleteLifecycle, ctx context.Context) {
				c.Quiesce(ctx)
			}, cmpQuiesceSkipMessage),
		Entry("stop",
			func(c CompleteLifecycle, ctx context.Context) {
				c.Stop(ctx)
			}, cmpStopSkipMessage),
	)

	It("keep a skipped component away from every registered interceptor", func() {
		interceptor := apimocks.NewMockQuiesceInterceptor(ctrl)

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains([]any{interceptor}).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Quiesce(context.Background())

		Expect(logs.records()).To(HaveLen(1))
	})

	It("sit outside the registered interceptors of a component that started", func() {
		var order []string

		first := apimocks.NewMockQuiesceInterceptor(ctrl)
		first.EXPECT().Quiesce(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Quiescer) {
			order = append(order, "first")
			next.Quiesce(ctx)
		})

		second := apimocks.NewMockQuiesceInterceptor(ctrl)
		second.EXPECT().Quiesce(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Quiescer) {
			order = append(order, "second")
			next.Quiesce(ctx)
		})

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any())
		comp.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
			order = append(order, "component")
		})

		wrapped := NewChains([]any{first, second}).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(Succeed())
		wrapped.Quiesce(context.Background())

		Expect(order).To(Equal([]string{"first", "second", "component"}))
		Expect(logs.records()).To(BeEmpty())
	})

	It("decide from the context alone, so one instance serves every component", func() {
		next := apimocks.NewMockQuiescer(ctrl)
		next.EXPECT().Quiesce(gomock.Any())

		QuiesceGate.Quiesce(withStatus(context.Background(), true), next)
		QuiesceGate.Quiesce(withStatus(context.Background(), false), next)

		Expect(logs.records()).To(HaveLen(1))
	})

	It("decide independently, so a failed component logs both skips", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))
		Expect(recs[0].Message).To(Equal(cmpQuiesceSkipMessage))
		Expect(recs[1].Message).To(Equal(cmpStopSkipMessage))
		Expect(attrs(recs[0])["component"].String()).To(Equal("ledger"))
		Expect(attrs(recs[1])["component"].String()).To(Equal("ledger"))
	})

	It("let a Wire cleanup run for a component whose Start failed", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errCmpBoom)

		cleaned := false
		wrapped := NewCleanableComponent(NewChains(nil).WrapComponent(comp), func() {
			cleaned = true
		})

		Expect(wrapped.Start(context.Background())).To(MatchError(errCmpBoom))
		wrapped.Stop(context.Background())

		Expect(cleaned).To(BeTrue())
		Expect(logs.records()).To(HaveLen(1))
	})

	It("pass a component that started cleanly through to both passes", func() {
		var quiesceStatus, stopStatus bool

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any())
		comp.EXPECT().Quiesce(gomock.Any()).Do(func(ctx context.Context) {
			quiesceStatus, _ = statusFromContext(ctx)
		})
		comp.EXPECT().Stop(gomock.Any()).Do(func(ctx context.Context) {
			stopStatus, _ = statusFromContext(ctx)
		})

		wrapped := NewChains(nil).WrapComponent(comp)

		Expect(wrapped.Start(context.Background())).To(Succeed())
		wrapped.Quiesce(context.Background())
		wrapped.Stop(context.Background())

		Expect(quiesceStatus).To(BeFalse())
		Expect(stopStatus).To(BeFalse())
		Expect(logs.records()).To(BeEmpty())
	})
})
