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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama"
	apimocks "l7e.io/yama/internal/mocks"
	execmocks "l7e.io/yama/rt/internal/mocks"
)

// callerKey keys a value the caller plants on the context it starts with.
type callerKey struct{}

// interceptorKey keys a value an interceptor adds to the context it substitutes.
type interceptorKey struct{}

// namedComponent gives a mock a fmt.Stringer identity. The mock supplies the
// behavior; this adds only the name.
type namedComponent struct {
	*execmocks.MockCompleteLifecycle

	name string
}

func (c *namedComponent) String() string { return c.name }

var _ = Describe("Overrun interceptor", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	Context("when the caller's context carries no deadline", func() {
		It("records nothing", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())

			Expect(NewChains(nil).WrapComponent(comp).Start(context.Background())).To(Succeed())

			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("when the operation returns before the deadline", func() {
		It("records nothing", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())

			ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
			DeferCleanup(cancel)

			Expect(NewChains(nil).WrapComponent(comp).Start(ctx)).To(Succeed())

			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("when the operation returns after the deadline", func() {
		It("warns once, naming the component and how far it ran over", func() {
			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "kafka-consumer",
			}
			comp.EXPECT().Start(gomock.Any())

			Expect(NewChains(nil).WrapComponent(comp).Start(expired())).To(Succeed())

			recs := logs.records()
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Level).To(Equal(slog.LevelWarn))
			Expect(recs[0].Message).To(Equal(overrunMessage))

			a := attrs(recs[0])
			Expect(a["component"].String()).To(Equal("kafka-consumer"))
			Expect(a["overrun"].Kind()).To(Equal(slog.KindDuration))
			Expect(a["overrun"].Duration()).To(BeNumerically(">", time.Duration(0)))
		})

		It("logs under the context the operation ran with", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())

			tagging := apimocks.NewMockStartInterceptor(ctrl)
			tagging.EXPECT().Start(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, next yama.Starter) error {
					return next.Start(context.WithValue(ctx, interceptorKey{}, "substituted"))
				})

			ctx := context.WithValue(expired(), callerKey{}, "tenant-7")

			Expect(NewChains([]any{tagging}).WrapComponent(comp).Start(ctx)).To(Succeed())

			ctxs := logs.contexts()
			Expect(ctxs).To(HaveLen(1))
			Expect(ctxs[0].Value(callerKey{})).To(Equal("tenant-7"))
			Expect(ctxs[0].Value(interceptorKey{})).To(Equal("substituted"))
		})

		It("identifies a component that is not a Stringer by its type", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Stop(gomock.Any())

			NewChains(nil).WrapComponent(comp).Stop(expired())

			recs := logs.records()
			Expect(recs).To(HaveLen(1))
			Expect(attrs(recs[0])["component"].String()).To(Equal("*mocks.MockCompleteLifecycle"))
		})

		It("waits for the operation instead of abandoning it", func() {
			const work = 50 * time.Millisecond

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				time.Sleep(work)
				return nil
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			DeferCleanup(cancel)

			began := time.Now()
			Expect(NewChains(nil).WrapComponent(comp).Start(ctx)).To(Succeed())

			Expect(time.Since(began)).To(BeNumerically(">=", work))
			Expect(logs.records()).To(HaveLen(1))
		})

		It("passes the component's error through unchanged", func() {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(boom)

			Expect(NewChains(nil).WrapComponent(comp).Start(expired())).To(MatchError(boom))

			Expect(logs.records()).To(HaveLen(1))
		})

		DescribeTable("reports the overrun whichever operation ran late",
			func(arrange func(*execmocks.MockCompleteLifecycle), invoke func(CompleteLifecycle, context.Context)) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				arrange(comp)

				invoke(NewChains(nil).WrapComponent(comp), expired())

				Expect(logs.records()).To(HaveLen(1))
			},
			Entry("start",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Start(gomock.Any())
				},
				func(c CompleteLifecycle, ctx context.Context) {
					_ = c.Start(ctx)
				}),
			Entry("quiesce",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Quiesce(gomock.Any())
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Quiesce(ctx)
				}),
			Entry("stop",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Stop(gomock.Any())
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Stop(ctx)
				}),
		)
	})
})
