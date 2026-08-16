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
	"fmt"
	"log/slog"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/bridge"
	apimocks "l7e.io/yama/v2/internal/mocks"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

// itcKey types the context keys these specs attach.
type itcKey string

const (
	itcCallerKey  itcKey = "caller"
	itcOuterKey   itcKey = "outer"
	itcInnerKey   itcKey = "inner"
	itcAttemptKey itcKey = "attempt"
)

// itcArrange arms the component mock for one operation, handing the context that
// operation receives to capture.
type itcArrange func(m *execmocks.MockCompleteLifecycle, capture func(context.Context))

// itcPanicArrange arms the component mock so one operation panics with v.
type itcPanicArrange func(m *execmocks.MockCompleteLifecycle, v any)

// itcIntercept builds a delegating interceptor for one operation. before runs on
// the context the interceptor receives and returns the context handed to next;
// after runs once next has returned.
type itcIntercept func(ctrl *gomock.Controller, before func(context.Context) context.Context, after func()) any

// itcBuild builds an interceptor for one operation.
type itcBuild func(ctrl *gomock.Controller) any

// itcInvoke runs one operation on a wrapped component.
type itcInvoke func(ctx context.Context, c CompleteLifecycle)

// itcPass hands the context to next unchanged.
func itcPass(ctx context.Context) context.Context { return ctx }

// itcNoop does nothing.
func itcNoop() {}

// itcComponent returns the component attached to ctx, or nil when ctx is nil or
// carries none.
func itcComponent(ctx context.Context) any {
	if ctx == nil {
		return nil
	}

	c, _ := yama.FromContext[any](ctx)

	return c
}

func itcArrangeStart(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
	m.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		capture(ctx)

		return nil
	})
}

func itcArrangeQuiesce(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
	m.EXPECT().Quiesce(gomock.Any()).Do(capture)
}

func itcArrangeStop(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
	m.EXPECT().Stop(gomock.Any()).Do(capture)
}

func itcPanicStart(m *execmocks.MockCompleteLifecycle, v any) {
	m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
		panic(v)
	})
}

func itcPanicQuiesce(m *execmocks.MockCompleteLifecycle, v any) {
	m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
		panic(v)
	})
}

func itcPanicStop(m *execmocks.MockCompleteLifecycle, v any) {
	m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
		panic(v)
	})
}

func itcInterceptStart(ctrl *gomock.Controller, before func(context.Context) context.Context, after func()) any {
	m := apimocks.NewMockStartInterceptor(ctrl)
	m.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) error {
		err := next.Start(before(ctx))
		after()

		return err
	})

	return m
}

func itcInterceptQuiesce(ctrl *gomock.Controller, before func(context.Context) context.Context, after func()) any {
	m := apimocks.NewMockQuiesceInterceptor(ctrl)
	m.EXPECT().Quiesce(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Quiescer) {
		next.Quiesce(before(ctx))
		after()
	})

	return m
}

func itcInterceptStop(ctrl *gomock.Controller, before func(context.Context) context.Context, after func()) any {
	m := apimocks.NewMockStopInterceptor(ctrl)
	m.EXPECT().Stop(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Stopper) {
		next.Stop(before(ctx))
		after()
	})

	return m
}

// itcSuppressStart returns a start interceptor that returns nil without calling
// next.
func itcSuppressStart(ctrl *gomock.Controller) any {
	m := apimocks.NewMockStartInterceptor(ctrl)
	m.EXPECT().Start(gomock.Any(), gomock.Any()).Return(nil)

	return m
}

// itcSuppressQuiesce returns a quiesce interceptor that returns without calling
// next.
func itcSuppressQuiesce(ctrl *gomock.Controller) any {
	m := apimocks.NewMockQuiesceInterceptor(ctrl)
	m.EXPECT().Quiesce(gomock.Any(), gomock.Any())

	return m
}

// itcSuppressStop returns a stop interceptor that returns without calling next.
func itcSuppressStop(ctrl *gomock.Controller) any {
	m := apimocks.NewMockStopInterceptor(ctrl)
	m.EXPECT().Stop(gomock.Any(), gomock.Any())

	return m
}

// itcSilentStart returns a start interceptor carrying no expectation, so any call
// to it fails the spec.
func itcSilentStart(ctrl *gomock.Controller) any {
	return apimocks.NewMockStartInterceptor(ctrl)
}

// itcSilentQuiesce returns a quiesce interceptor carrying no expectation, so any
// call to it fails the spec.
func itcSilentQuiesce(ctrl *gomock.Controller) any {
	return apimocks.NewMockQuiesceInterceptor(ctrl)
}

// itcSilentStop returns a stop interceptor carrying no expectation, so any call
// to it fails the spec.
func itcSilentStop(ctrl *gomock.Controller) any {
	return apimocks.NewMockStopInterceptor(ctrl)
}

func itcInvokeStart(ctx context.Context, c CompleteLifecycle) {
	_ = c.Start(ctx)
}

func itcInvokeQuiesce(ctx context.Context, c CompleteLifecycle) {
	c.Quiesce(ctx)
}

func itcInvokeStop(ctx context.Context, c CompleteLifecycle) {
	c.Stop(ctx)
}

var _ = Describe("Interceptor view of the wrapped component", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	It("hands an interceptor the exact component that was wrapped", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any())

		var (
			seen *namedComponent
			ok   bool
		)

		i := itcInterceptStart(ctrl, func(ctx context.Context) context.Context {
			seen, ok = yama.FromContext[*namedComponent](ctx)

			return ctx
		}, itcNoop)

		Expect(NewChains([]any{i}).WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(ok).To(BeTrue())
		Expect(seen).To(BeIdenticalTo(comp))
	})

	DescribeTable("exposes that same instance on every operation",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "ledger",
			}

			var componentSaw, interceptorSaw context.Context

			arrange(comp.MockCompleteLifecycle, func(ctx context.Context) { componentSaw = ctx })
			i := intercept(ctrl, func(ctx context.Context) context.Context {
				interceptorSaw = ctx

				return ctx
			}, itcNoop)

			invoke(context.Background(), NewChains([]any{i}).WrapComponent(comp))

			Expect(itcComponent(interceptorSaw)).To(BeIdenticalTo(comp))
			Expect(itcComponent(componentSaw)).To(BeIdenticalTo(comp))
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("attaches identity with no interceptors registered at all",
		func(arrange itcArrange, invoke itcInvoke) {
			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "ledger",
			}

			var componentSaw context.Context

			arrange(comp.MockCompleteLifecycle, func(ctx context.Context) { componentSaw = ctx })

			invoke(context.Background(), NewChains(nil).WrapComponent(comp))

			Expect(itcComponent(componentSaw)).To(BeIdenticalTo(comp))
		},
		Entry("start", itcArrangeStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInvokeStop),
	)

	It("attaches the identity ahead of the outermost interceptor", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any())

		var outerSaw any

		outer := itcInterceptStart(ctrl, func(ctx context.Context) context.Context {
			outerSaw = itcComponent(ctx)

			return ctx
		}, itcNoop)
		inner := itcInterceptStart(ctrl, itcPass, itcNoop)

		Expect(NewChains([]any{outer, inner}).WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(outerSaw).To(BeIdenticalTo(comp))
	})

	It("shadows an identity the caller attached before the call", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any())

		var seen any

		i := itcInterceptStart(ctrl, func(ctx context.Context) context.Context {
			seen = itcComponent(ctx)

			return ctx
		}, itcNoop)

		ctx := bridge.WithComponent(context.Background(), "caller-attached-identity")

		Expect(NewChains([]any{i}).WrapComponent(comp).Start(ctx)).To(Succeed())

		Expect(seen).To(BeIdenticalTo(comp))
	})

	It("never hands the component itself to an interceptor as next", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any())

		var next yama.Starter

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, n yama.Starter) error {
			next = n

			return n.Start(ctx)
		})

		Expect(NewChains([]any{i}).WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(next).NotTo(BeIdenticalTo(comp))
		_, isComponent := next.(*namedComponent)
		Expect(isComponent).To(BeFalse())
		_, isStringer := next.(fmt.Stringer)
		Expect(isStringer).To(BeFalse())
	})

	It("keeps the identities of two concurrently driven components apart", func() {
		var (
			mu       sync.Mutex
			byMethod = map[*namedComponent]any{}
			byChain  []any
		)

		record := func(comp *namedComponent) func(context.Context) error {
			return func(ctx context.Context) error {
				mu.Lock()
				defer mu.Unlock()

				byMethod[comp] = itcComponent(ctx)

				return nil
			}
		}

		arrived := make(chan struct{}, 2)
		release := make(chan struct{})

		shared := apimocks.NewMockStartInterceptor(ctrl)
		shared.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) error {
			mu.Lock()
			byChain = append(byChain, itcComponent(ctx))
			mu.Unlock()

			arrived <- struct{}{}
			<-release

			return next.Start(ctx)
		}).Times(2)

		chains := NewChains([]any{shared})

		alpha := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "alpha",
		}
		beta := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "beta",
		}
		alpha.EXPECT().Start(gomock.Any()).DoAndReturn(record(alpha))
		beta.EXPECT().Start(gomock.Any()).DoAndReturn(record(beta))

		wrapped := []CompleteLifecycle{chains.WrapComponent(alpha), chains.WrapComponent(beta)}
		errs := make([]error, len(wrapped))

		var wg sync.WaitGroup

		for i, w := range wrapped {
			wg.Add(1)
			go func(i int, w CompleteLifecycle) {
				defer wg.Done()
				errs[i] = w.Start(context.Background())
			}(i, w)
		}

		Eventually(arrived).Should(HaveLen(2))
		close(release)
		wg.Wait()

		Expect(errs).To(HaveEach(BeNil()))
		Expect(byChain).To(ConsistOf(BeIdenticalTo(alpha), BeIdenticalTo(beta)))
		Expect(byMethod[alpha]).To(BeIdenticalTo(alpha))
		Expect(byMethod[beta]).To(BeIdenticalTo(beta))
	})
})

var _ = Describe("Interceptor control of the operation context", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	DescribeTable("carries an interceptor's context edit to everything inner",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			var componentSaw, innerSaw context.Context

			arrange(comp, func(ctx context.Context) { componentSaw = ctx })
			outer := intercept(ctrl, func(ctx context.Context) context.Context {
				return context.WithValue(ctx, itcOuterKey, "outer-value")
			}, itcNoop)
			inner := intercept(ctrl, func(ctx context.Context) context.Context {
				innerSaw = ctx

				return ctx
			}, itcNoop)

			invoke(context.Background(), NewChains([]any{outer, inner}).WrapComponent(comp))

			Expect(innerSaw.Value(itcOuterKey)).To(Equal("outer-value"))
			Expect(componentSaw.Value(itcOuterKey)).To(Equal("outer-value"))
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("carries the caller's own context values through to the component",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			var componentSaw context.Context

			arrange(comp, func(ctx context.Context) { componentSaw = ctx })
			i := intercept(ctrl, itcPass, itcNoop)

			ctx := context.WithValue(context.Background(), itcCallerKey, "caller-value")
			invoke(ctx, NewChains([]any{i}).WrapComponent(comp))

			Expect(componentSaw.Value(itcCallerKey)).To(Equal("caller-value"))
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("keeps an inner interceptor's context edit from the enclosing one",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			var (
				componentSaw, outerSaw context.Context
				outerAfter             any
			)

			arrange(comp, func(ctx context.Context) { componentSaw = ctx })
			outer := intercept(ctrl, func(ctx context.Context) context.Context {
				outerSaw = ctx

				return ctx
			}, func() { outerAfter = outerSaw.Value(itcInnerKey) })
			inner := intercept(ctrl, func(ctx context.Context) context.Context {
				return context.WithValue(ctx, itcInnerKey, "inner-value")
			}, itcNoop)

			invoke(context.Background(), NewChains([]any{outer, inner}).WrapComponent(comp))

			Expect(componentSaw.Value(itcInnerKey)).To(Equal("inner-value"))
			Expect(outerAfter).To(BeNil())
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("hands the component exactly the context an interceptor substituted",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			var componentSaw context.Context

			arrange(comp, func(ctx context.Context) { componentSaw = ctx })
			i := intercept(ctrl, func(context.Context) context.Context {
				return context.Background()
			}, itcNoop)

			ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
			DeferCleanup(cancel)

			invoke(context.WithValue(ctx, itcCallerKey, "caller-value"), NewChains([]any{i}).WrapComponent(comp))

			Expect(itcComponent(componentSaw)).To(BeNil())
			Expect(componentSaw.Value(itcCallerKey)).To(BeNil())
			_, hasDeadline := componentSaw.Deadline()
			Expect(hasDeadline).To(BeFalse())
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	It("delivers the caller's own deadline and cancellation to the component", func() {
		deadline := time.Now().Add(time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		DeferCleanup(cancel)

		var (
			componentDeadline time.Time
			hasDeadline       bool
			cancellation      error
		)

		entered := make(chan struct{})

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			componentDeadline, hasDeadline = ctx.Deadline()
			close(entered)
			<-ctx.Done()
			cancellation = ctx.Err()

			return nil
		})

		i := itcInterceptStart(ctrl, itcPass, itcNoop)
		w := NewChains([]any{i}).WrapComponent(comp)

		done := make(chan error, 1)
		go func() {
			done <- w.Start(ctx)
		}()

		Eventually(entered).Should(BeClosed())
		cancel()
		Eventually(done).Should(Receive(BeNil()))

		Expect(hasDeadline).To(BeTrue())
		Expect(componentDeadline).To(BeTemporally("==", deadline))
		Expect(cancellation).To(MatchError(context.Canceled))
	})
})

var _ = Describe("Interceptor control of the operation outcome", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	DescribeTable("suppresses everything inner when an interceptor never calls next",
		func(suppress, silent itcBuild, invoke itcInvoke) {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			invoke(expired(), NewChains([]any{suppress(ctrl), silent(ctrl)}).WrapComponent(comp))

			Expect(logs.records()).To(BeEmpty())
		},
		Entry("start", itcSuppressStart, itcSilentStart, itcInvokeStart),
		Entry("quiesce", itcSuppressQuiesce, itcSilentQuiesce, itcInvokeQuiesce),
		Entry("stop", itcSuppressStop, itcSilentStop, itcInvokeStop),
	)

	It("reports the suppressing start interceptor's success as success", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)

		Expect(NewChains([]any{itcSuppressStart(ctrl)}).WrapComponent(comp).Start(context.Background())).To(Succeed())
	})

	It("returns the suppressing start interceptor's error verbatim", func() {
		refused := errors.New("refused")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).Return(refused)

		err := NewChains([]any{i}).WrapComponent(comp).Start(context.Background())

		Expect(err).To(BeIdenticalTo(refused))
	})

	It("routes only the capabilities the component implements", func() {
		stopper := apimocks.NewMockStopper(ctrl)
		stopper.EXPECT().Stop(gomock.Any())

		stopped := false
		interceptors := []any{
			itcSilentStart(ctrl),
			itcSilentQuiesce(ctrl),
			itcInterceptStop(ctrl, itcPass, func() { stopped = true }),
		}

		w := NewChains(interceptors).WrapComponent(stopper)

		Expect(w.Start(context.Background())).To(Succeed())

		w.Quiesce(context.Background())
		w.Stop(context.Background())

		Expect(stopped).To(BeTrue())
	})

	It("wraps a value implementing no capability into an inert lifecycle", func() {
		interceptors := []any{itcSilentStart(ctrl), itcSilentQuiesce(ctrl), itcSilentStop(ctrl)}

		w := NewChains(interceptors).WrapComponent(inertComponent{})

		Expect(w.Start(context.Background())).To(Succeed())
		Expect(func() {
			w.Quiesce(context.Background())
			w.Stop(context.Background())
		}).NotTo(Panic())
	})

	//nolint:dupl // mirrors chains_test.go's chain-level case; different subject under test.
	It("lets a start interceptor turn the component's failure into a started component", func() {
		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))
		comp.EXPECT().Quiesce(gomock.Any())
		comp.EXPECT().Stop(gomock.Any())

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) error {
			_ = next.Start(ctx)

			return nil
		})

		w := NewChains([]any{i}).WrapComponent(comp)

		Expect(w.Start(context.Background())).To(Succeed())

		w.Quiesce(context.Background())
		w.Stop(context.Background())
	})

	It("lets a start interceptor turn a clean start into a failed component", func() {
		refused := errors.New("refused")

		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any())

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) error {
			_ = next.Start(ctx)

			return refused
		})

		w := NewChains([]any{i}).WrapComponent(comp)

		Expect(w.Start(context.Background())).To(BeIdenticalTo(refused))

		w.Quiesce(context.Background())
		w.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))

		for _, r := range recs {
			Expect(r.Level).To(Equal(slog.LevelWarn))
			Expect(attrs(r)["component"].String()).To(Equal("ledger"))
		}
	})

	It("keeps a failed component's own interceptors out of the shutdown passes", func() {
		comp := &namedComponent{
			MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
			name:                  "ledger",
		}
		comp.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

		interceptors := []any{itcSilentQuiesce(ctrl), itcSilentStop(ctrl)}

		w := NewChains(interceptors).WrapComponent(comp)

		Expect(w.Start(context.Background())).NotTo(Succeed())

		w.Quiesce(context.Background())
		w.Stop(context.Background())

		recs := logs.records()
		Expect(recs).To(HaveLen(2))

		for _, r := range recs {
			Expect(attrs(r)["component"].String()).To(Equal("ledger"))
		}
	})

	It("runs the component once per invocation when an interceptor calls next again", func() {
		var attempts []string

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context) error {
			attempt, _ := ctx.Value(itcAttemptKey).(string)
			attempts = append(attempts, attempt)

			if len(attempts) == 1 {
				return errors.New("boom")
			}

			return nil
		})

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) error {
			if err := next.Start(context.WithValue(ctx, itcAttemptKey, "first")); err != nil {
				return next.Start(context.WithValue(ctx, itcAttemptKey, "second"))
			}

			return nil
		})

		Expect(NewChains([]any{i}).WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(attempts).To(Equal([]string{"first", "second"}))
	})

	It("measures a span covering the whole operation, the outer one covering the inner", func() {
		const work = 50 * time.Millisecond

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			time.Sleep(work)

			return nil
		})

		var (
			outerBegan, innerBegan time.Time
			outerSpan, innerSpan   time.Duration
		)

		outer := itcInterceptStart(ctrl, func(ctx context.Context) context.Context {
			outerBegan = time.Now()

			return ctx
		}, func() { outerSpan = time.Since(outerBegan) })

		inner := itcInterceptStart(ctrl, func(ctx context.Context) context.Context {
			innerBegan = time.Now()

			return ctx
		}, func() { innerSpan = time.Since(innerBegan) })

		Expect(NewChains([]any{outer, inner}).WrapComponent(comp).Start(context.Background())).To(Succeed())

		Expect(innerSpan).To(BeNumerically(">=", work))
		Expect(outerSpan).To(BeNumerically(">=", innerSpan))
	})
})

var _ = Describe("Chain panic layering", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	DescribeTable("propagates a panic raised by the component",
		func(arrange itcPanicArrange, invoke itcInvoke) {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp, boom)

			w := NewChains(nil).WrapComponent(comp)

			Expect(func() { invoke(context.Background(), w) }).To(PanicWith(BeIdenticalTo(boom)))
		},
		Entry("start", itcPanicStart, itcInvokeStart),
		Entry("quiesce", itcPanicQuiesce, itcInvokeQuiesce),
		Entry("stop", itcPanicStop, itcInvokeStop),
	)

	DescribeTable("propagates a panic raised by an interceptor before it delegates",
		func(intercept itcIntercept, invoke itcInvoke) {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			i := intercept(ctrl, func(context.Context) context.Context {
				panic(boom)
			}, itcNoop)

			w := NewChains([]any{i}).WrapComponent(comp)

			Expect(func() { invoke(context.Background(), w) }).To(PanicWith(BeIdenticalTo(boom)))
		},
		Entry("start", itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("propagates a panic raised by an interceptor after next returns",
		func(arrange itcArrange, intercept itcIntercept, invoke itcInvoke) {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp, func(context.Context) {})
			i := intercept(ctrl, itcPass, func() {
				panic(boom)
			})

			w := NewChains([]any{i}).WrapComponent(comp)

			Expect(func() { invoke(context.Background(), w) }).To(PanicWith(BeIdenticalTo(boom)))
		},
		Entry("start", itcArrangeStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcArrangeQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcArrangeStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("abandons an enclosing interceptor's post-next work",
		func(arrange itcPanicArrange, intercept itcIntercept, invoke itcInvoke) {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp, boom)

			var trace []string

			outer := intercept(ctrl, func(ctx context.Context) context.Context {
				trace = append(trace, "outer entered")

				return ctx
			}, func() { trace = append(trace, "outer resumed") })
			inner := intercept(ctrl, func(ctx context.Context) context.Context {
				trace = append(trace, "inner entered")

				return ctx
			}, func() { trace = append(trace, "inner resumed") })

			w := NewChains([]any{outer, inner}).WrapComponent(comp)

			Expect(func() { invoke(context.Background(), w) }).To(PanicWith(BeIdenticalTo(boom)))
			Expect(trace).To(Equal([]string{"outer entered", "inner entered"}))
		},
		Entry("start", itcPanicStart, itcInterceptStart, itcInvokeStart),
		Entry("quiesce", itcPanicQuiesce, itcInterceptQuiesce, itcInvokeQuiesce),
		Entry("stop", itcPanicStop, itcInterceptStop, itcInvokeStop),
	)

	DescribeTable("records no overrun for an operation that panicked past its deadline",
		func(arrange itcPanicArrange, invoke itcInvoke) {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			arrange(comp, boom)

			w := NewChains(nil).WrapComponent(comp)

			Expect(func() { invoke(expired(), w) }).To(PanicWith(BeIdenticalTo(boom)))
			Expect(logs.records()).To(BeEmpty())
		},
		Entry("start", itcPanicStart, itcInvokeStart),
		Entry("quiesce", itcPanicQuiesce, itcInvokeQuiesce),
		Entry("stop", itcPanicStop, itcInvokeStop),
	)

	It("lets a start interceptor recover a panic and report its own error", func() {
		boom := errors.New("boom")
		salvaged := errors.New("salvaged")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		itcPanicStart(comp, boom)

		var caught any

		i := apimocks.NewMockStartInterceptor(ctrl)
		i.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next yama.Starter) (err error) {
			defer func() {
				if r := recover(); r != nil {
					caught = r
					err = salvaged
				}
			}()

			return next.Start(ctx)
		})

		var err error

		Expect(func() {
			err = NewChains([]any{i}).WrapComponent(comp).Start(context.Background())
		}).NotTo(Panic())

		Expect(caught).To(BeIdenticalTo(boom))
		Expect(err).To(BeIdenticalTo(salvaged))
	})

	It("lets a stop interceptor recover a panic so nothing escapes the wrapper", func() {
		boom := errors.New("boom")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		itcPanicStop(comp, boom)

		var caught any

		i := apimocks.NewMockStopInterceptor(ctrl)
		i.EXPECT().Stop(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Stopper) {
			defer func() { caught = recover() }()

			next.Stop(ctx)
		})

		w := NewChains([]any{i}).WrapComponent(comp)

		Expect(func() { w.Stop(context.Background()) }).NotTo(Panic())

		Expect(caught).To(BeIdenticalTo(boom))
	})

	It("leaves recovery to the level that holds the same wrapped component", func() {
		boom := errors.New("boom")

		comp := execmocks.NewMockCompleteLifecycle(ctrl)
		comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
			panic(boom)
		}).Times(2)

		w := NewChains(nil).WrapComponent(comp)

		Expect(func() { _ = w.Start(context.Background()) }).To(PanicWith(BeIdenticalTo(boom)))

		var err error

		Expect(func() { err = Level{w}.Start(context.Background()) }).NotTo(Panic())

		Expect(err).To(MatchError(yama.ErrStartFailed))

		recs := logs.records()
		Expect(recs).To(HaveLen(1))
		Expect(recs[0].Level).To(Equal(slog.LevelError))
	})
})
