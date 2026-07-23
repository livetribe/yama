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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama/v2"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

// errLcBoom is the failure a level member reports from its Start.
var errLcBoom = errors.New("member start failed")

// lcCall is one operation a member observed, together with the context it was
// handed.
type lcCall struct {
	label string
	ctx   context.Context
}

// lcRecorder collects the operations a spec's members observe, in the order
// they were entered.
type lcRecorder struct {
	mu    sync.Mutex
	calls []lcCall
}

func (r *lcRecorder) add(label string, ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, lcCall{label: label, ctx: ctx})
}

// labels returns the labels of everything recorded so far, in order.
func (r *lcRecorder) labels() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		out = append(out, c.label)
	}

	return out
}

// contexts returns the contexts observed by the recorded calls carrying one of
// the given labels.
func (r *lcRecorder) contexts(labels ...string) []context.Context {
	want := make(map[string]bool, len(labels))
	for _, l := range labels {
		want[l] = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]context.Context, 0, len(r.calls))
	for _, c := range r.calls {
		if want[c.label] {
			out = append(out, c.ctx)
		}
	}

	return out
}

// lcHeld returns a member that records every operation it observes against rec
// under name. Its Start returns startErr. A non-nil holdStart or holdQuiesce
// parks the corresponding operation until that channel is closed.
func lcHeld(
	ctrl *gomock.Controller,
	rec *lcRecorder,
	name string,
	startErr error,
	holdStart, holdQuiesce <-chan struct{},
) *execmocks.MockCompleteLifecycle {
	m := execmocks.NewMockCompleteLifecycle(ctrl)

	m.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		rec.add(name+".start", ctx)

		if holdStart != nil {
			<-holdStart
		}

		return startErr
	}).AnyTimes()

	m.EXPECT().Quiesce(gomock.Any()).Do(func(ctx context.Context) {
		rec.add(name+".quiesce", ctx)

		if holdQuiesce != nil {
			<-holdQuiesce
		}
	}).AnyTimes()

	m.EXPECT().Stop(gomock.Any()).Do(func(ctx context.Context) {
		rec.add(name+".stop", ctx)
	}).AnyTimes()

	return m
}

// lcMember returns a member that records against rec and never parks.
func lcMember(ctrl *gomock.Controller, rec *lcRecorder, name string, startErr error) *execmocks.MockCompleteLifecycle {
	return lcHeld(ctrl, rec, name, startErr, nil, nil)
}

// lcGraph builds n single-member levels named L0..Ln-1. The member of level
// failAt fails its Start; an out-of-range failAt leaves every level succeeding.
func lcGraph(ctrl *gomock.Controller, rec *lcRecorder, n, failAt int) []Level {
	levels := make([]Level, n)

	for i := range levels {
		var err error
		if i == failAt {
			err = errLcBoom
		}

		levels[i] = Level{lcMember(ctrl, rec, fmt.Sprintf("L%d", i), err)}
	}

	return levels
}

// lcAsync runs fn on its own goroutine and returns a channel closed when fn
// returns.
func lcAsync(fn func()) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer GinkgoRecover()
		defer close(done)

		fn()
	}()

	return done
}

// lcPanics runs fn on its own goroutine and delivers the value it panicked
// with, or nil if it returned. An fn that blocks delivers nothing.
func lcPanics(fn func()) <-chan any {
	out := make(chan any, 1)

	go func() {
		defer func() {
			out <- recover()
		}()

		fn()
	}()

	return out
}

var _ = Describe("Lifecycle state machine", func() {
	var (
		ctrl *gomock.Controller
		rec  *lcRecorder
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		rec = &lcRecorder{}
		ctx = context.Background()
	})

	Describe("construction", func() {
		It("returns a lifecycle that is immediately startable", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))

			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L1.start"}))
		})

		It("returns a value that exposes no Quiesce to its caller", func() {
			_, ok := NewLifecycle(lcGraph(ctrl, rec, 1, -1)).(yama.Quiescer)

			Expect(ok).To(BeFalse())
		})
	})

	Describe("startup traversal", func() {
		It("runs the levels forward, once each, and touches neither shutdown pass", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))

			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L1.start", "L2.start"}))
		})

		It("starts and stops a lifecycle holding a single level", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 1, -1))

			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{"L0.start"}))

			lc.Stop(ctx)

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L0.quiesce", "L0.stop"}))
		})

		It("starts and restarts a lifecycle holding no levels", func() {
			lc := NewLifecycle(nil)

			Expect(lc.Start(ctx)).To(Succeed())
			Expect(func() {
				lc.Stop(ctx)
			}).NotTo(Panic())

			Expect(lc.Start(ctx)).To(Succeed())
			Expect(func() {
				lc.Stop(ctx)
			}).NotTo(Panic())
		})
	})

	Describe("startup failure", func() {
		It("halts at the failing level and tears down everything reached, including it", func() {
			levels := append(lcGraph(ctrl, rec, 3, 2), Level{execmocks.NewMockCompleteLifecycle(ctrl)})
			lc := NewLifecycle(levels)

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start", "L2.start",
				"L2.quiesce", "L1.quiesce", "L0.quiesce",
				"L2.stop", "L1.stop", "L0.stop",
			}))
		})

		It("quiesces and tears down the first level when that is the one that failed", func() {
			levels := []Level{
				{lcMember(ctrl, rec, "L0", errLcBoom)},
				{execmocks.NewMockCompleteLifecycle(ctrl)},
				{execmocks.NewMockCompleteLifecycle(ctrl)},
			}
			lc := NewLifecycle(levels)

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L0.quiesce", "L0.stop"}))
		})

		It("tears down the sole level when the whole graph is that one level", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 1, 0))

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			settled := []string{"L0.start", "L0.quiesce", "L0.stop"}
			Expect(rec.labels()).To(Equal(settled))

			lc.Stop(ctx)

			Expect(rec.labels()).To(Equal(settled))
		})

		It("counts positions over the levels slice as given, empty levels included", func() {
			levels := []Level{
				{lcMember(ctrl, rec, "L0", nil)},
				{},
				{lcMember(ctrl, rec, "L2", errLcBoom)},
				{execmocks.NewMockCompleteLifecycle(ctrl)},
			}
			lc := NewLifecycle(levels)

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L2.start",
				"L2.quiesce", "L0.quiesce",
				"L2.stop", "L0.stop",
			}))
		})
	})

	Describe("the started state", func() {
		It("makes a second Start a no-op", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))

			Expect(lc.Start(ctx)).To(Succeed())
			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L1.start", "L2.start"}))
		})
	})

	Describe("the error state", func() {
		var (
			lc      yama.Lifecycle
			settled []string
		)

		BeforeEach(func() {
			lc = NewLifecycle(lcGraph(ctrl, rec, 3, 1))

			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			settled = rec.labels()
			Expect(settled).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
			}))
		})

		It("fails a later Start without re-running any level", func() {
			Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			Expect(rec.labels()).To(Equal(settled))
		})

		It("does not repeat the teardown a later Stop would otherwise run", func() {
			lc.Stop(ctx)

			Expect(rec.labels()).To(Equal(settled))
		})

		It("never returns to a running state, whatever the caller does next", func() {
			for range 3 {
				lc.Stop(ctx)
				Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))
			}

			Expect(rec.labels()).To(Equal(settled))
		})
	})

	Describe("a start context already canceled", func() {
		It("declines to start, running no level, and reports ErrStartFailed", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))

			Expect(lc.Start(expired())).To(MatchError(yama.ErrStartFailed))
			Expect(rec.labels()).To(BeEmpty())
		})

		It("stays startable, so a later Start under a live context runs every level", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))

			Expect(lc.Start(expired())).To(MatchError(yama.ErrStartFailed))
			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L1.start", "L2.start"}))
		})
	})

	Describe("a start deadline reached mid-startup", func() {
		It("treats a component's own deadline error as an ordinary start failure", func() {
			// base comes up; waiter honors the caller's context and fails when the
			// deadline fires while it is starting. Yama does not preempt waiter or
			// convert the deadline into an error of its own — it waits for waiter's
			// own ctx.Err() and then cleans up, as any start failure does.
			base := lcMember(ctrl, rec, "base", nil)

			waiter := execmocks.NewMockCompleteLifecycle(ctrl)
			waiter.EXPECT().Start(gomock.Any()).DoAndReturn(func(c context.Context) error {
				rec.add("waiter.start", c)
				<-c.Done()

				return c.Err()
			})
			waiter.EXPECT().Quiesce(gomock.Any()).Do(func(c context.Context) {
				rec.add("waiter.quiesce", c)
			})
			waiter.EXPECT().Stop(gomock.Any()).Do(func(c context.Context) {
				rec.add("waiter.stop", c)
			})

			deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()

			lc := NewLifecycle([]Level{{base}, {waiter}})

			Expect(lc.Start(deadline)).To(MatchError(yama.ErrStartFailed))

			Expect(rec.labels()).To(Equal([]string{
				"base.start", "waiter.start",
				"waiter.quiesce", "base.quiesce",
				"waiter.stop", "base.stop",
			}))
		})
	})

	Describe("shutdown", func() {
		It("does nothing for a lifecycle that was never started", func() {
			NewLifecycle(lcGraph(ctrl, rec, 2, -1)).Stop(ctx)

			Expect(rec.labels()).To(BeEmpty())
		})

		It("walks the levels backward and completes the quiesce pass before any teardown", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))
			Expect(lc.Start(ctx)).To(Succeed())

			lc.Stop(ctx)

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start", "L2.start",
				"L2.quiesce", "L1.quiesce", "L0.quiesce",
				"L2.stop", "L1.stop", "L0.stop",
			}))
		})

		It("runs the two passes exactly once however often it is called", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))
			Expect(lc.Start(ctx)).To(Succeed())

			for range 3 {
				lc.Stop(ctx)
			}

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
			}))
		})
	})

	Describe("restart", func() {
		It("runs every level forward again after a completed Stop", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))

			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)
			Expect(lc.Start(ctx)).To(Succeed())

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
				"L0.start", "L1.start",
			}))
		})

		It("re-arms both shutdown passes", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))

			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)
			Expect(lc.Start(ctx)).To(Succeed())
			lc.Stop(ctx)

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
			}))
		})
	})

	Describe("the uninitialized value", func() {
		DescribeTable("panics without touching a single level",
			func(drive func(*lifecycle)) {
				lc := &lifecycle{levels: []Level{{execmocks.NewMockCompleteLifecycle(ctrl)}}}

				Expect(func() {
					drive(lc)
				}).To(PanicWith("uninitialized lifecycle"))
			},
			Entry("on Start", func(lc *lifecycle) {
				_ = lc.Start(context.Background())
			}),
			Entry("on Stop", func(lc *lifecycle) {
				lc.Stop(context.Background())
			}),
		)

		It("releases the mutex as it panics, so later calls panic rather than block", func() {
			lc := &lifecycle{levels: []Level{{execmocks.NewMockCompleteLifecycle(ctrl)}}}

			Expect(func() {
				_ = lc.Start(ctx)
			}).To(PanicWith("uninitialized lifecycle"))

			Eventually(lcPanics(func() {
				_ = lc.Start(ctx)
			})).Should(Receive(Equal("uninitialized lifecycle")))

			Eventually(lcPanics(func() {
				lc.Stop(ctx)
			})).Should(Receive(Equal("uninitialized lifecycle")))
		})
	})

	Describe("context propagation", func() {
		type lcKey struct{}

		It("hands every level the caller's own Start context", func() {
			startCtx := context.WithValue(ctx, lcKey{}, "start")
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))

			Expect(lc.Start(startCtx)).To(Succeed())

			seen := rec.contexts("L0.start", "L1.start")
			Expect(seen).To(HaveLen(2))
			Expect(seen).To(HaveEach(BeIdenticalTo(startCtx)))
		})

		It("hands every quiesce and teardown the caller's own Stop context", func() {
			stopCtx := context.WithValue(ctx, lcKey{}, "stop")
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, -1))
			Expect(lc.Start(ctx)).To(Succeed())

			lc.Stop(stopCtx)

			seen := rec.contexts("L1.quiesce", "L0.quiesce", "L1.stop", "L0.stop")
			Expect(seen).To(HaveLen(4))
			Expect(seen).To(HaveEach(BeIdenticalTo(stopCtx)))
		})

		It("runs startup-failure cleanup under the failing Start's context", func() {
			startCtx := context.WithValue(ctx, lcKey{}, "start")
			lc := NewLifecycle(lcGraph(ctrl, rec, 2, 1))

			Expect(lc.Start(startCtx)).To(MatchError(yama.ErrStartFailed))

			seen := rec.contexts("L0.start", "L1.start", "L1.quiesce", "L0.quiesce", "L1.stop", "L0.stop")
			Expect(seen).To(HaveLen(6))
			Expect(seen).To(HaveEach(BeIdenticalTo(startCtx)))
		})

		It("runs both shutdown passes despite a Stop context that already expired", func() {
			lc := NewLifecycle(lcGraph(ctrl, rec, 3, -1))
			Expect(lc.Start(ctx)).To(Succeed())

			lc.Stop(expired())

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start", "L2.start",
				"L2.quiesce", "L1.quiesce", "L0.quiesce",
				"L2.stop", "L1.stop", "L0.stop",
			}))
		})
	})

	Describe("serialization", func() {
		It("makes a Stop issued during a Start wait for that Start", func() {
			release := make(chan struct{})
			levels := []Level{
				{lcMember(ctrl, rec, "L0", nil)},
				{lcHeld(ctrl, rec, "L1", nil, release, nil)},
				{lcMember(ctrl, rec, "L2", nil)},
			}
			lc := NewLifecycle(levels)

			starting := lcAsync(func() {
				Expect(lc.Start(ctx)).To(Succeed())
			})
			Eventually(rec.labels).Should(Equal([]string{"L0.start", "L1.start"}))

			stopping := lcAsync(func() {
				lc.Stop(ctx)
			})
			Consistently(rec.labels).Should(Equal([]string{"L0.start", "L1.start"}))

			close(release)
			Eventually(starting).Should(BeClosed())
			Eventually(stopping).Should(BeClosed())

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start", "L2.start",
				"L2.quiesce", "L1.quiesce", "L0.quiesce",
				"L2.stop", "L1.stop", "L0.stop",
			}))
		})

		It("does not repeat the teardown for a Stop issued during a failing Start", func() {
			release := make(chan struct{})
			levels := []Level{
				{lcMember(ctrl, rec, "L0", nil)},
				{lcHeld(ctrl, rec, "L1", errLcBoom, release, nil)},
				{execmocks.NewMockCompleteLifecycle(ctrl)},
			}
			lc := NewLifecycle(levels)

			starting := lcAsync(func() {
				Expect(lc.Start(ctx)).To(MatchError(yama.ErrStartFailed))
			})
			Eventually(rec.labels).Should(Equal([]string{"L0.start", "L1.start"}))

			stopping := lcAsync(func() {
				lc.Stop(ctx)
			})
			Consistently(rec.labels).Should(Equal([]string{"L0.start", "L1.start"}))

			close(release)
			Eventually(starting).Should(BeClosed())
			Eventually(stopping).Should(BeClosed())

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
			}))
		})

		It("makes a Start issued during a Stop wait, then restarts the levels", func() {
			release := make(chan struct{})
			levels := []Level{
				{lcMember(ctrl, rec, "L0", nil)},
				{lcHeld(ctrl, rec, "L1", nil, nil, release)},
			}
			lc := NewLifecycle(levels)
			Expect(lc.Start(ctx)).To(Succeed())

			stopping := lcAsync(func() {
				lc.Stop(ctx)
			})
			Eventually(rec.labels).Should(Equal([]string{"L0.start", "L1.start", "L1.quiesce"}))

			starting := lcAsync(func() {
				Expect(lc.Start(ctx)).To(Succeed())
			})
			Consistently(rec.labels).Should(Equal([]string{"L0.start", "L1.start", "L1.quiesce"}))

			close(release)
			Eventually(stopping).Should(BeClosed())
			Eventually(starting).Should(BeClosed())

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
				"L0.start", "L1.start",
			}))
		})

		It("starts each level exactly once across concurrent Start callers", func() {
			const callers = 4

			release := make(chan struct{})
			levels := []Level{
				{lcHeld(ctrl, rec, "L0", nil, release, nil)},
				{lcMember(ctrl, rec, "L1", nil)},
			}
			lc := NewLifecycle(levels)

			errs := make(chan error, callers)
			running := []<-chan struct{}{lcAsync(func() {
				errs <- lc.Start(ctx)
			})}
			Eventually(rec.labels).Should(Equal([]string{"L0.start"}))

			for range callers - 1 {
				running = append(running, lcAsync(func() {
					errs <- lc.Start(ctx)
				}))
			}
			Consistently(rec.labels).Should(Equal([]string{"L0.start"}))

			close(release)
			for _, done := range running {
				Eventually(done).Should(BeClosed())
			}

			close(errs)
			for err := range errs {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(rec.labels()).To(Equal([]string{"L0.start", "L1.start"}))
		})

		It("runs the shutdown passes exactly once across concurrent Stop callers", func() {
			const callers = 4

			release := make(chan struct{})
			levels := []Level{
				{lcMember(ctrl, rec, "L0", nil)},
				{lcHeld(ctrl, rec, "L1", nil, nil, release)},
			}
			lc := NewLifecycle(levels)
			Expect(lc.Start(ctx)).To(Succeed())

			running := []<-chan struct{}{lcAsync(func() {
				lc.Stop(ctx)
			})}
			Eventually(rec.labels).Should(Equal([]string{"L0.start", "L1.start", "L1.quiesce"}))

			for range callers - 1 {
				running = append(running, lcAsync(func() {
					lc.Stop(ctx)
				}))
			}
			Consistently(rec.labels).Should(Equal([]string{"L0.start", "L1.start", "L1.quiesce"}))

			close(release)
			for _, done := range running {
				Eventually(done).Should(BeClosed())
			}

			Expect(rec.labels()).To(Equal([]string{
				"L0.start", "L1.start",
				"L1.quiesce", "L0.quiesce",
				"L1.stop", "L0.stop",
			}))
		})
	})
})
