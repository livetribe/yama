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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/mocks"
	"l7e.io/yama/v2/rt"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

// A call a spec never sets an expectation for fails the moment it happens, so
// "must never be torn down" is expressed by an absent Stop expectation rather
// than by asserting a zero count afterward. Level members run on their own
// goroutines, so specs record what a callback observed and assert only after
// the synchronizing Start or Stop has returned.
var _ = Describe("LifecycleBuilder", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	Describe("boundaries", func() {
		It("bracket the graph as its dependency extremes, so shutdown reverses startup exactly", func() {
			begin := execmocks.NewMockCompleteLifecycle(ctrl)
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)
			end := execmocks.NewMockCompleteLifecycle(ctrl)

			gomock.InOrder(
				begin.EXPECT().Start(gomock.Any()), base.EXPECT().Start(gomock.Any()),
				top.EXPECT().Start(gomock.Any()), end.EXPECT().Start(gomock.Any()),
				end.EXPECT().Quiesce(gomock.Any()), top.EXPECT().Quiesce(gomock.Any()),
				base.EXPECT().Quiesce(gomock.Any()), begin.EXPECT().Quiesce(gomock.Any()),
				end.EXPECT().Stop(gomock.Any()), top.EXPECT().Stop(gomock.Any()),
				base.EXPECT().Stop(gomock.Any()), begin.EXPECT().Stop(gomock.Any()),
			)

			b := rt.NewLifecycleBuilder(yama.WithBeginComponents(begin), yama.WithEndComponents(end))
			b.NextLevel().WithComponents(base).Add()
			b.NextLevel().WithComponents(top).Add()
			lc := b.Build()

			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)
		})

		It("participate only in the passes their type implements", func() {
			beginStarter := mocks.NewMockStarter(ctrl)
			endStopper := mocks.NewMockStopper(ctrl)
			only := execmocks.NewMockCompleteLifecycle(ctrl)

			beginStarter.EXPECT().Start(gomock.Any())
			endStopper.EXPECT().Stop(gomock.Any())
			only.EXPECT().Start(gomock.Any())
			only.EXPECT().Quiesce(gomock.Any())
			only.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder(yama.WithBeginComponents(beginStarter), yama.WithEndComponents(endStopper))
			b.NextLevel().WithComponents(only).Add()
			lc := b.Build()

			Expect(lc.Start(ctx)).To(Succeed())
			Expect(func() { lc.Stop(ctx) }).NotTo(Panic())
		})

		Context("a Starter boundary that fails", func() {
			It("stops the graph from starting, as the begin boundary", func() {
				begin := execmocks.NewMockCompleteLifecycle(ctrl)
				graph := execmocks.NewMockCompleteLifecycle(ctrl)

				begin.EXPECT().Start(gomock.Any()).Return(errors.New("begin boom"))

				b := rt.NewLifecycleBuilder(yama.WithBeginComponents(begin))
				b.NextLevel().WithComponents(graph).Add()

				Expect(b.Build().Start(ctx)).To(MatchError(yama.ErrStartFailed))
			})

			It("cleans up the started graph, as the end boundary", func() {
				end := execmocks.NewMockCompleteLifecycle(ctrl)
				graph := execmocks.NewMockCompleteLifecycle(ctrl)

				graph.EXPECT().Start(gomock.Any())
				end.EXPECT().Start(gomock.Any()).Return(errors.New("end boom"))
				graph.EXPECT().Quiesce(gomock.Any())
				graph.EXPECT().Stop(gomock.Any())

				b := rt.NewLifecycleBuilder(yama.WithEndComponents(end))
				b.NextLevel().WithComponents(graph).Add()

				Expect(b.Build().Start(ctx)).To(MatchError(yama.ErrStartFailed))
			})
		})

		It("take no part in cleanup when the failing start never reached them", func() {
			// end has no Stop expectation, so tearing it down would fail the spec.
			begin := mocks.NewMockStopper(ctrl)
			end := mocks.NewMockStopper(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)

			begin.EXPECT().Stop(gomock.Any())
			failing.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

			b := rt.NewLifecycleBuilder(yama.WithBeginComponents(begin), yama.WithEndComponents(end))
			b.NextLevel().WithComponents(failing).Add()

			Expect(b.Build().Start(ctx)).To(MatchError(yama.ErrStartFailed))
		})
	})

	Describe("Wire cleanup functions", func() {
		It("run before the owning component's Stop", func() {
			owner := execmocks.NewMockCompleteLifecycle(ctrl)

			cleaned := false
			stopSawCleanup := false
			owner.EXPECT().Start(gomock.Any())
			owner.EXPECT().Quiesce(gomock.Any())
			owner.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				stopSawCleanup = cleaned
			})

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponentAndCleanup(owner, func() { cleaned = true }).Add()
			lc := b.Build()

			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)

			Expect(stopSawCleanup).To(BeTrue(), "the cleanup must run before the component's Stop")
			Expect(cleaned).To(BeTrue())
		})

		It("run even when the component's Start failed", func() {
			// The owner has no Stop expectation, so tearing it down would fail the spec.
			owner := execmocks.NewMockCompleteLifecycle(ctrl)

			cleaned := false
			owner.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponentAndCleanup(owner, func() { cleaned = true }).Add()
			lc := b.Build()

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))
			Expect(cleaned).To(BeTrue(), "the cleanup runs even though Start failed")
		})

		It("bypass the interceptor chain, which sees only the component's own Stop", func() {
			owner := execmocks.NewMockCompleteLifecycle(ctrl)
			interceptor := mocks.NewMockStopInterceptor(ctrl)

			owner.EXPECT().Start(gomock.Any())
			owner.EXPECT().Quiesce(gomock.Any())
			owner.EXPECT().Stop(gomock.Any())
			interceptor.EXPECT().Stop(gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, next yama.Stopper) { next.Stop(ctx) }).
				Times(1)

			b := rt.NewLifecycleBuilder(yama.WithInterceptors(interceptor))
			b.NextLevel().WithComponentAndCleanup(owner, func() {}).Add()
			lc := b.Build()

			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)
		})
	})

	Describe("interceptors", func() {
		It("run in registration order and reach the component last", func() {
			only := execmocks.NewMockCompleteLifecycle(ctrl)
			first := mocks.NewMockStartInterceptor(ctrl)
			second := mocks.NewMockStartInterceptor(ctrl)

			delegate := func(ctx context.Context, next yama.Starter) error { return next.Start(ctx) }

			gomock.InOrder(
				first.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(delegate),
				second.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(delegate),
				only.EXPECT().Start(gomock.Any()),
			)

			b := rt.NewLifecycleBuilder(yama.WithInterceptors(first, second))
			b.NextLevel().WithComponents(only).Add()

			Expect(b.Build().Start(ctx)).To(Succeed())
		})

		It("attach to every component, with no per-component scoping", func() {
			first := execmocks.NewMockCompleteLifecycle(ctrl)
			second := execmocks.NewMockCompleteLifecycle(ctrl)
			third := execmocks.NewMockCompleteLifecycle(ctrl)
			interceptor := mocks.NewMockStartInterceptor(ctrl)

			for _, m := range []*execmocks.MockCompleteLifecycle{first, second, third} {
				m.EXPECT().Start(gomock.Any())
			}
			interceptor.EXPECT().Start(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, next yama.Starter) error { return next.Start(ctx) }).
				Times(3)

			b := rt.NewLifecycleBuilder(yama.WithInterceptors(interceptor))
			b.NextLevel().WithComponents(first, second).Add()
			b.NextLevel().WithComponents(third).Add()

			Expect(b.Build().Start(ctx)).To(Succeed())
		})
	})

	Describe("misuse", func() {
		It("panics on any use of a builder after Build", func() {
			b := rt.NewLifecycleBuilder()
			b.Build()

			Expect(func() { b.NextLevel() }).To(Panic())
			Expect(func() { b.Build() }).To(Panic())
		})

		It("panics on any use of a level builder after Add", func() {
			lb := rt.NewLifecycleBuilder().NextLevel()
			lb.Add()

			Expect(func() { lb.Add() }).To(Panic())
			Expect(func() { lb.WithComponents(execmocks.NewMockCompleteLifecycle(ctrl)) }).To(Panic())
		})
	})
})
