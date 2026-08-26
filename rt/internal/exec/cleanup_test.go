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

	"l7e.io/yama/v2"
	apimocks "l7e.io/yama/v2/internal/mocks"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

// clnRecorder is an ordered log of what a spec observed.
type clnRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *clnRecorder) record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)
}

// taken returns a snapshot of everything recorded so far.
func (r *clnRecorder) taken() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.events...)
}

// recording builds a cleanup function that logs event when it runs.
func (r *clnRecorder) recording(event string) func() {
	return func() {
		r.record(event)
	}
}

// clnMeetTimeout bounds a rendezvous. Members that never overlap fail the spec
// rather than hang it.
const clnMeetTimeout = 2 * time.Second

// clnMeet announces this member's arrival and waits for the other's, reporting
// whether the two were in flight at the same time.
func clnMeet(mine, theirs chan struct{}) bool {
	close(mine)

	select {
	case <-theirs:
		return true
	case <-time.After(clnMeetTimeout):
		return false
	}
}

var _ = Describe("A Wire cleanup paired with a component", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
		rec  *clnRecorder
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
		rec = &clnRecorder{}
		ctx = context.Background()
	})

	Context("the startup pass", func() {
		It("reports the component's own error, neither wrapped nor replaced", func() {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(boom)

			err := NewCleanableComponent(comp, rec.recording("cleanup")).Start(ctx)

			Expect(err).To(BeIdenticalTo(boom))
		})

		DescribeTable("never releases the provider's resources",
			func(outcome error) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				comp.EXPECT().Start(gomock.Any()).Return(outcome)

				_ = NewCleanableComponent(comp, rec.recording("cleanup")).Start(ctx)

				Expect(rec.taken()).To(BeEmpty())
			},
			Entry("when the component starts", nil),
			Entry("when the component fails to start", errors.New("boom")),
		)
	})

	Context("the quiesce pass", func() {
		It("delegates to the component and releases nothing", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
				rec.record("component")
			})

			NewCleanableComponent(comp, rec.recording("cleanup")).Quiesce(ctx)

			Expect(rec.taken()).To(Equal([]string{"component"}))
		})
	})

	Context("the release pass", func() {
		It("runs the cleanup and gives the component no call", func() {
			// comp carries no expectations, so any call to it fails the spec.
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			NewCleanableComponent(comp, rec.recording("cleanup")).Release(ctx)

			Expect(rec.taken()).To(Equal([]string{"cleanup"}))
		})
	})

	Context("the teardown pass", func() {
		It("releases the provider's resources before the outermost stop interceptor is entered", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())
			comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				rec.record("component")
			})

			interceptor := apimocks.NewMockStopInterceptor(ctrl)
			interceptor.EXPECT().Stop(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, next yama.Stopper) {
				rec.record("interceptor")
				next.Stop(ctx)
			})

			pair := NewCleanableComponent(NewChains([]any{interceptor}).WrapComponent(comp), rec.recording("cleanup"))

			Expect(pair.Start(ctx)).To(Succeed())
			pair.Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"cleanup", "interceptor", "component"}))
		})

		DescribeTable("keeps that order whatever the component contributes",
			func(build func() (CompleteLifecycle, []string)) {
				pair, want := build()

				Expect(pair.Start(ctx)).To(Succeed())
				pair.Stop(ctx)

				Expect(rec.taken()).To(Equal(want))
			},
			Entry("a component that implements nothing", func() (CompleteLifecycle, []string) {
				pair := NewCleanableComponent(NewChains(nil).WrapComponent(struct{}{}), rec.recording("cleanup"))

				return pair, []string{"cleanup"}
			}),
			Entry("a component that only starts", func() (CompleteLifecycle, []string) {
				comp := apimocks.NewMockStarter(ctrl)
				comp.EXPECT().Start(gomock.Any())

				pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

				return pair, []string{"cleanup"}
			}),
			Entry("a component that only stops", func() (CompleteLifecycle, []string) {
				comp := apimocks.NewMockStopper(ctrl)
				comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
					rec.record("component")
				})

				pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

				return pair, []string{"cleanup", "component"}
			}),
			Entry("a component that implements every operation", func() (CompleteLifecycle, []string) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				comp.EXPECT().Start(gomock.Any())
				comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
					rec.record("component")
				})

				pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

				return pair, []string{"cleanup", "component"}
			}),
		)

		It("releases the provider's resources again on every Stop", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())
			comp.EXPECT().Stop(gomock.Any()).Times(2).Do(func(context.Context) {
				rec.record("component")
			})

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

			Expect(pair.Start(ctx)).To(Succeed())
			pair.Stop(ctx)
			pair.Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"cleanup", "component", "cleanup", "component"}))
		})

		It("runs the outer cleanup, then the inner one, then the component", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				rec.record("component")
			})

			inner := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("inner"))

			NewCleanableComponent(inner, rec.recording("outer")).Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"outer", "inner", "component"}))
		})
	})

	Context("a component whose Start failed", func() {
		It("releases the provider's resources while the gate drops the component's teardown", func() {
			boom := errors.New("boom")

			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "postgres-pool",
			}
			comp.EXPECT().Start(gomock.Any()).Return(boom)

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

			Expect(pair.Start(ctx)).To(MatchError(boom))
			pair.Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"cleanup"}))

			recs := logs.records()
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Level).To(Equal(slog.LevelWarn))
			Expect(recs[0].Message).To(Equal("component startup failed, skipping stop"))
			Expect(attrs(recs[0])["component"].String()).To(Equal("postgres-pool"))
		})

		It("releases them even when the component contributes no teardown", func() {
			boom := errors.New("boom")

			comp := apimocks.NewMockStarter(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(boom)

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

			Expect(pair.Start(ctx)).To(MatchError(boom))
			pair.Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"cleanup"}))
			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("a component that started cleanly", func() {
		It("joins both shutdown passes with no gate record", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any())
			comp.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
				rec.record("quiesce")
			})
			comp.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				rec.record("stop")
			})

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), rec.recording("cleanup"))

			Expect(pair.Start(ctx)).To(Succeed())
			pair.Quiesce(ctx)
			pair.Stop(ctx)

			Expect(rec.taken()).To(Equal([]string{"quiesce", "cleanup", "stop"}))
			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("the caller's context", func() {
		DescribeTable("reaches the component untouched",
			func(arrange func(*execmocks.MockCompleteLifecycle, func(context.Context)), invoke func(CompleteLifecycle, context.Context)) {
				var seen context.Context

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				arrange(comp, func(c context.Context) {
					seen = c
				})

				caller, cancel := context.WithTimeout(context.Background(), time.Hour)
				DeferCleanup(cancel)

				invoke(NewCleanableComponent(comp, rec.recording("cleanup")), caller)

				Expect(seen).To(BeIdenticalTo(caller))
			},
			Entry("on start",
				func(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
					m.EXPECT().Start(gomock.Any()).DoAndReturn(func(c context.Context) error {
						capture(c)

						return nil
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					_ = c.Start(ctx)
				}),
			Entry("on quiesce",
				func(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
					m.EXPECT().Quiesce(gomock.Any()).Do(capture)
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Quiesce(ctx)
				}),
			Entry("on stop",
				func(m *execmocks.MockCompleteLifecycle, capture func(context.Context)) {
					m.EXPECT().Stop(gomock.Any()).Do(capture)
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Stop(ctx)
				}),
		)

		It("charges a cleanup that outlasts the deadline to the component", func() {
			const (
				budget = 10 * time.Millisecond
				work   = 30 * time.Millisecond
			)

			comp := &namedComponent{
				MockCompleteLifecycle: execmocks.NewMockCompleteLifecycle(ctrl),
				name:                  "blob-store",
			}
			comp.EXPECT().Start(gomock.Any())
			comp.EXPECT().Stop(gomock.Any())

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), func() {
				time.Sleep(work)
			})
			Expect(pair.Start(ctx)).To(Succeed())

			deadlined, cancel := context.WithTimeout(context.Background(), budget)
			DeferCleanup(cancel)
			pair.Stop(deadlined)

			recs := logs.records()
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Level).To(Equal(slog.LevelWarn))
			Expect(recs[0].Message).To(Equal(overrunMessage))

			a := attrs(recs[0])
			Expect(a["component"].String()).To(Equal("blob-store"))
			Expect(a["overrun"].Duration()).To(BeNumerically(">=", work-budget))
		})

		It("charges it to nobody when the component contributes no teardown", func() {
			const (
				budget = 10 * time.Millisecond
				work   = 30 * time.Millisecond
			)

			comp := apimocks.NewMockStarter(ctrl)
			comp.EXPECT().Start(gomock.Any())

			pair := NewCleanableComponent(NewChains(nil).WrapComponent(comp), func() {
				time.Sleep(work)
			})
			Expect(pair.Start(ctx)).To(Succeed())

			deadlined, cancel := context.WithTimeout(context.Background(), budget)
			DeferCleanup(cancel)
			pair.Stop(deadlined)

			Expect(logs.records()).To(BeEmpty())
		})
	})

	Context("when something panics", func() {
		It("propagates a panicking cleanup and never reaches the component", func() {
			comp := execmocks.NewMockCompleteLifecycle(ctrl)

			pair := NewCleanableComponent(comp, func() {
				panic("cleanup exploded")
			})

			Expect(func() { pair.Stop(ctx) }).To(PanicWith("cleanup exploded"))
		})

		DescribeTable("propagates a panicking component",
			func(arrange func(*execmocks.MockCompleteLifecycle), invoke func(CompleteLifecycle, context.Context)) {
				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				arrange(comp)

				pair := NewCleanableComponent(comp, rec.recording("cleanup"))

				Expect(func() { invoke(pair, ctx) }).To(PanicWith("component exploded"))
			},
			Entry("from start",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
						panic("component exploded")
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					_ = c.Start(ctx)
				}),
			Entry("from quiesce",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
						panic("component exploded")
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Quiesce(ctx)
				}),
			Entry("from stop",
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
						panic("component exploded")
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Stop(ctx)
				}),
		)
	})

	Context("as a level member", func() {
		DescribeTable("runs alongside its siblings, and the level waits for both",
			func(arrange func(*execmocks.MockCompleteLifecycle, func()), invoke func(CompleteLifecycle, context.Context)) {
				paired := make(chan struct{})
				sibling := make(chan struct{})
				met := make(chan bool, 2)

				comp := execmocks.NewMockCompleteLifecycle(ctrl)
				arrange(comp, func() {
					met <- clnMeet(paired, sibling)
				})

				other := execmocks.NewMockCompleteLifecycle(ctrl)
				arrange(other, func() {
					met <- clnMeet(sibling, paired)
				})

				invoke(Level{NewCleanableComponent(comp, rec.recording("cleanup")), other}, ctx)

				Expect(met).To(HaveLen(2))
				Expect(<-met).To(BeTrue())
				Expect(<-met).To(BeTrue())
			},
			Entry("through the startup pass",
				func(m *execmocks.MockCompleteLifecycle, meet func()) {
					m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
						meet()

						return nil
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					_ = c.Start(ctx)
				}),
			Entry("through the quiesce pass",
				func(m *execmocks.MockCompleteLifecycle, meet func()) {
					m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
						meet()
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Quiesce(ctx)
				}),
			Entry("through the teardown pass",
				func(m *execmocks.MockCompleteLifecycle, meet func()) {
					m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
						meet()
					})
				},
				func(c CompleteLifecycle, ctx context.Context) {
					c.Stop(ctx)
				}),
		)

		It("fails the level's startup while its siblings still finish", func() {
			boom := errors.New("boom")

			comp := execmocks.NewMockCompleteLifecycle(ctrl)
			comp.EXPECT().Start(gomock.Any()).Return(boom)

			other := execmocks.NewMockCompleteLifecycle(ctrl)
			other.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				rec.record("sibling")

				return nil
			})

			level := Level{NewCleanableComponent(comp, rec.recording("cleanup")), other}

			Expect(level.Start(ctx)).To(MatchError(yama.ErrStartFailed))
			Expect(rec.taken()).To(Equal([]string{"sibling"}))
		})
	})
})

var _ = Describe("A standalone Wire cleanup", func() {
	var (
		ctrl *gomock.Controller
		rec  *clnRecorder
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		rec = &clnRecorder{}
		ctx = context.Background()
	})

	It("takes no part in the startup pass", func() {
		Expect(Cleanup(rec.recording("cleanup")).Start(ctx)).To(Succeed())
		Expect(rec.taken()).To(BeEmpty())
	})

	It("takes no part in the quiesce pass", func() {
		Cleanup(rec.recording("cleanup")).Quiesce(ctx)

		Expect(rec.taken()).To(BeEmpty())
	})

	It("runs the cleanup as its whole teardown", func() {
		Cleanup(rec.recording("cleanup")).Stop(ctx)

		Expect(rec.taken()).To(Equal([]string{"cleanup"}))
	})

	It("runs the cleanup on Release, exactly as on Stop", func() {
		Cleanup(rec.recording("cleanup")).Release(ctx)

		Expect(rec.taken()).To(Equal([]string{"cleanup"}))
	})

	// A standalone cleanup never reaches the stop gate, so it still releases what
	// its provider acquired even though a sibling's failed start gates that
	// sibling's own teardown away.
	It("releases the provider's resources when a sibling's start failed", func() {
		boom := errors.New("boom")

		comp := apimocks.NewMockStarter(ctrl)
		comp.EXPECT().Start(gomock.Any()).Return(boom)

		sibling := NewChains(nil).WrapComponent(comp)
		level := Level{sibling, Cleanup(rec.recording("cleanup"))}

		Expect(level.Start(ctx)).To(MatchError(yama.ErrStartFailed))
		level.Stop(ctx)

		Expect(rec.taken()).To(Equal([]string{"cleanup"}))
	})

	It("propagates a panicking cleanup to the level runner", func() {
		Expect(func() {
			Cleanup(func() { panic("cleanup exploded") }).Stop(ctx)
		}).To(PanicWith("cleanup exploded"))
	})

	It("runs concurrently with the other members of its level", func() {
		mine, theirs := make(chan struct{}), make(chan struct{})
		met := make(chan bool, 2)

		other := execmocks.NewMockCompleteLifecycle(ctrl)
		other.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
			met <- clnMeet(theirs, mine)
		})

		level := Level{Cleanup(func() { met <- clnMeet(mine, theirs) }), other}
		level.Stop(ctx)

		Expect(met).To(HaveLen(2))
		Expect(<-met).To(BeTrue())
		Expect(<-met).To(BeTrue())
	})
})
