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
	"maps"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama/v2"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

const (
	// lvlPanicMessage is the message a recovered panic is logged under.
	lvlPanicMessage = "panic recovered"

	// lvlBound caps a rendezvous, and any spec that would otherwise hang rather
	// than fail. It sits above goroutine scheduling latency and nothing more.
	lvlBound = time.Second

	// lvlGrace is how long a member watches its context for a cancellation
	// that must never arrive.
	lvlGrace = 50 * time.Millisecond

	// lvlSettle is how long a member works before recording its completion.
	lvlSettle = 25 * time.Millisecond
)

// errLvlBoom is what a failing member reports or panics with.
var errLvlBoom = errors.New("lvl boom")

// lvlCtxKey keys the value that distinguishes a spec's own context.
type lvlCtxKey struct{}

// lvlPass drives one of the three passes over a Level.
type lvlPass struct {
	// expect arranges a member to run body when the pass reaches it.
	expect func(m *execmocks.MockCompleteLifecycle, body func(ctx context.Context))

	// invoke runs the pass, discarding anything it reports.
	invoke func(l Level, ctx context.Context)
}

var lvlStartPass = lvlPass{
	expect: func(m *execmocks.MockCompleteLifecycle, body func(ctx context.Context)) {
		m.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			body(ctx)

			return nil
		})
	},
	invoke: func(l Level, ctx context.Context) {
		_ = l.Start(ctx)
	},
}

var lvlQuiescePass = lvlPass{
	expect: func(m *execmocks.MockCompleteLifecycle, body func(ctx context.Context)) {
		m.EXPECT().Quiesce(gomock.Any()).Do(func(ctx context.Context) {
			body(ctx)
		})
	},
	invoke: func(l Level, ctx context.Context) {
		l.Quiesce(ctx)
	},
}

var lvlStopPass = lvlPass{
	expect: func(m *execmocks.MockCompleteLifecycle, body func(ctx context.Context)) {
		m.EXPECT().Stop(gomock.Any()).Do(func(ctx context.Context) {
			body(ctx)
		})
	},
	invoke: func(l Level, ctx context.Context) {
		l.Stop(ctx)
	},
}

// lvlMember returns a member arranged to run body when pass reaches it.
func lvlMember(ctrl *gomock.Controller, pass lvlPass, body func(ctx context.Context)) CompleteLifecycle {
	m := execmocks.NewMockCompleteLifecycle(ctrl)
	pass.expect(m, body)

	return m
}

// lvlMembers returns one member per name, each running body under its own name.
func lvlMembers(
	ctrl *gomock.Controller,
	pass lvlPass,
	names []string,
	body func(name string, ctx context.Context),
) Level {
	lv := make(Level, 0, len(names))

	for _, name := range names {
		lv = append(lv, lvlMember(ctrl, pass, func(ctx context.Context) {
			body(name, ctx)
		}))
	}

	return lv
}

// lvlErupt panics with v. Specs match the logged stack against this frame.
func lvlErupt(v any) {
	panic(v)
}

// lvlRecorder collects what member bodies observed. Members run on goroutines
// the pass owns; a spec records here and asserts once the pass has returned.
type lvlRecorder struct {
	mu   sync.Mutex
	seen map[string]int
	ctxs map[string]context.Context
}

func lvlNewRecorder() *lvlRecorder {
	return &lvlRecorder{
		seen: make(map[string]int),
		ctxs: make(map[string]context.Context),
	}
}

func (r *lvlRecorder) note(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen[name]++
}

func (r *lvlRecorder) noteCtx(name string, ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ctxs[name] = ctx
}

func (r *lvlRecorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.seen[name]
}

func (r *lvlRecorder) counts() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return maps.Clone(r.seen)
}

func (r *lvlRecorder) ctxFor(name string) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.ctxs[name]
}

// lvlGate holds every arriving member until all of them have arrived. It
// releases unconditionally once lvlBound elapses, so members run one after
// another fail their spec instead of wedging it. Every caller of arrive must
// assert on what it reports.
type lvlGate struct {
	remaining atomic.Int32
	open      chan struct{}
	lapse     sync.Once
	lapsed    chan struct{}
}

func lvlNewGate(members int) *lvlGate {
	g := &lvlGate{
		open:   make(chan struct{}),
		lapsed: make(chan struct{}),
	}
	g.remaining.Store(int32(members)) //nolint:gosec // members is a small test-fixture count.

	return g
}

// arrive blocks until every member has arrived and reports whether that
// happened within lvlBound. Once one arrival has waited that long, every later
// arrival gives up at once.
func (g *lvlGate) arrive() bool {
	if g.remaining.Add(-1) == 0 {
		close(g.open)
	}

	select {
	case <-g.open:
		return true
	case <-g.lapsed:
		return false
	case <-time.After(lvlBound):
		g.lapse.Do(func() {
			close(g.lapsed)
		})

		return false
	}
}

var _ = Describe("Level", func() {
	var (
		ctrl *gomock.Controller
		logs *captureHandler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		logs = captureSlog()
	})

	Context("with no members", func() {
		DescribeTable("runs every pass as a clean no-op",
			func(lv Level) {
				Expect(lv.Start(context.Background())).To(Succeed())

				Expect(func() {
					lv.Quiesce(context.Background())
				}).NotTo(Panic())

				Expect(func() {
					lv.Stop(context.Background())
				}).NotTo(Panic())

				Expect(logs.records()).To(BeEmpty())
			},
			Entry("nil", Level(nil)),
			Entry("empty", Level{}),
		)
	})

	Context("running its members", func() {
		DescribeTable("has every member in flight at the same time",
			func(pass lvlPass) {
				gate := lvlNewGate(3)
				rec := lvlNewRecorder()

				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo", "charlie"},
					func(name string, _ context.Context) {
						if gate.arrive() {
							rec.note(name)
						}
					})

				pass.invoke(lv, context.Background())

				Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1, "charlie": 1}))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		DescribeTable("returns only after every member's call has returned",
			func(pass lvlPass) {
				rec := lvlNewRecorder()

				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo", "charlie"},
					func(name string, _ context.Context) {
						time.Sleep(lvlSettle)

						rec.note(name)
					})

				pass.invoke(lv, context.Background())

				Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1, "charlie": 1}))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		DescribeTable("invokes each member exactly once and no other operation",
			func(pass lvlPass) {
				rec := lvlNewRecorder()

				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo", "charlie", "delta"},
					func(name string, _ context.Context) {
						rec.note(name)
					})

				pass.invoke(lv, context.Background())

				Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1, "charlie": 1, "delta": 1}))
				Expect(logs.records()).To(BeEmpty())
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		DescribeTable("hands every member the very context it was given",
			func(pass lvlPass) {
				rec := lvlNewRecorder()
				ctx := context.WithValue(context.Background(), lvlCtxKey{}, "spec")

				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo"},
					func(name string, got context.Context) {
						rec.noteCtx(name, got)
					})

				pass.invoke(lv, ctx)

				Expect(rec.ctxFor("alpha")).To(BeIdenticalTo(ctx))
				Expect(rec.ctxFor("bravo")).To(BeIdenticalTo(ctx))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)
	})

	Context("reporting a Start", func() {
		It("succeeds when every member starts", func() {
			rec := lvlNewRecorder()

			lv := lvlMembers(ctrl, lvlStartPass, []string{"alpha", "bravo"},
				func(name string, _ context.Context) {
					rec.note(name)
				})

			Expect(lv.Start(context.Background())).To(Succeed())

			Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1}))
		})

		It("reports ErrStartFailed when a member errors, running the rest anyway", func() {
			rec := lvlNewRecorder()

			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			failing.EXPECT().Start(gomock.Any()).Return(errLvlBoom)

			siblings := lvlMembers(ctrl, lvlStartPass, []string{"alpha", "bravo"},
				func(name string, _ context.Context) {
					time.Sleep(lvlSettle)

					rec.note(name)
				})

			lv := append(Level{failing}, siblings...)

			Expect(lv.Start(context.Background())).To(MatchError(yama.ErrStartFailed))

			Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1}))
			Expect(logs.records()).To(BeEmpty())
		})

		It("never carries the member's own error", func() {
			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			failing.EXPECT().Start(gomock.Any()).Return(errLvlBoom)

			err := Level{failing}.Start(context.Background())

			Expect(errors.Is(err, yama.ErrStartFailed)).To(BeTrue())
			Expect(errors.Is(err, errLvlBoom)).To(BeFalse())
		})

		It("collapses an error, a panic and a success into one ErrStartFailed", func() {
			rec := lvlNewRecorder()

			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			failing.EXPECT().Start(gomock.Any()).Return(errLvlBoom)

			panicking := lvlMember(ctrl, lvlStartPass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})

			succeeding := lvlMember(ctrl, lvlStartPass, func(context.Context) {
				time.Sleep(lvlSettle)

				rec.note("succeeding")
			})

			err := Level{failing, panicking, succeeding}.Start(context.Background())

			Expect(err).To(MatchError(yama.ErrStartFailed))
			Expect(errors.Is(err, errLvlBoom)).To(BeFalse())
			Expect(rec.count("succeeding")).To(Equal(1))
			Expect(logs.records()).To(HaveLen(1))
		})

		It("reports one failure when many members fail at once", func() {
			const members = 24

			gate := lvlNewGate(members)
			met := make(chan bool, members)
			lv := make(Level, 0, members)

			for i := range members {
				if i%2 == 0 {
					erroring := execmocks.NewMockCompleteLifecycle(ctrl)
					erroring.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
						met <- gate.arrive()

						return errLvlBoom
					})

					lv = append(lv, erroring)

					continue
				}

				lv = append(lv, lvlMember(ctrl, lvlStartPass, func(context.Context) {
					met <- gate.arrive()

					lvlErupt(errLvlBoom)
				}))
			}

			Expect(lv.Start(context.Background())).To(MatchError(yama.ErrStartFailed))

			Expect(met).To(HaveLen(members))

			for range members {
				Expect(<-met).To(BeTrue())
			}

			Expect(logs.records()).To(HaveLen(members / 2))
		})

		DescribeTable("decides each Start on that pass alone",
			func(arrangeFirst func(m *execmocks.MockCompleteLifecycle)) {
				member := execmocks.NewMockCompleteLifecycle(ctrl)
				arrangeFirst(member)
				member.EXPECT().Start(gomock.Any()).Return(nil)

				lv := Level{member}

				Expect(lv.Start(context.Background())).To(MatchError(yama.ErrStartFailed))
				Expect(lv.Start(context.Background())).To(Succeed())
			},
			Entry("after a member errored", func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Start(gomock.Any()).Return(errLvlBoom)
			}),
			Entry("after a member panicked", func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
					lvlErupt(errLvlBoom)

					return nil
				})
			}),
		)
	})

	Context("when a member panics", func() {
		It("reports the panic as a start failure and logs it, in the one call", func() {
			lv := Level{lvlMember(ctrl, lvlStartPass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})}

			err := lv.Start(context.Background())

			recs := logs.records()

			Expect(err).To(MatchError(yama.ErrStartFailed))
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Message).To(Equal(lvlPanicMessage))
		})

		It("logs the panic under the caller's context", func() {
			ctx := context.WithValue(context.Background(), lvlCtxKey{}, "spec")

			lv := Level{lvlMember(ctrl, lvlStartPass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})}

			Expect(lv.Start(ctx)).To(MatchError(yama.ErrStartFailed))

			ctxs := logs.contexts()
			Expect(ctxs).To(HaveLen(1))
			Expect(ctxs[0].Value(lvlCtxKey{})).To(Equal("spec"))
		})

		DescribeTable("returns to its caller without panicking",
			func(pass lvlPass) {
				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo"},
					func(_ string, _ context.Context) {
						lvlErupt(errLvlBoom)
					})

				Expect(func() {
					pass.invoke(lv, context.Background())
				}).NotTo(Panic())

				Expect(logs.records()).To(HaveLen(2))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		DescribeTable("runs every other member to completion",
			func(pass lvlPass) {
				rec := lvlNewRecorder()

				panicking := lvlMember(ctrl, pass, func(context.Context) {
					lvlErupt(errLvlBoom)
				})

				siblings := lvlMembers(ctrl, pass, []string{"alpha", "bravo"},
					func(name string, _ context.Context) {
						time.Sleep(lvlSettle)

						rec.note(name)
					})

				pass.invoke(append(Level{panicking}, siblings...), context.Background())

				Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1}))
				Expect(logs.records()).To(HaveLen(1))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		DescribeTable("logs one record per panicking member",
			func(pass lvlPass) {
				panicking := lvlMembers(ctrl, pass, []string{"alpha", "bravo", "charlie"},
					func(_ string, _ context.Context) {
						lvlErupt(errLvlBoom)
					})

				quiet := lvlMembers(ctrl, pass, []string{"delta", "echo"},
					func(_ string, _ context.Context) {})

				pass.invoke(append(panicking, quiet...), context.Background())

				Expect(logs.records()).To(HaveLen(3))
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		It("finishes every pass despite the panic", func(ctx SpecContext) {
			names := []string{"alpha", "bravo"}

			erupt := func(_ string, _ context.Context) {
				lvlErupt(errLvlBoom)
			}

			Expect(lvlMembers(ctrl, lvlStartPass, names, erupt).Start(ctx)).To(MatchError(yama.ErrStartFailed))
			lvlMembers(ctrl, lvlQuiescePass, names, erupt).Quiesce(ctx)
			lvlMembers(ctrl, lvlStopPass, names, erupt).Stop(ctx)

			Expect(logs.records()).To(HaveLen(6))
		}, SpecTimeout(lvlBound))

		It("logs at Error under the panic message, before the pass returns", func() {
			lv := Level{lvlMember(ctrl, lvlQuiescePass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})}

			lv.Quiesce(context.Background())

			recs := logs.records()

			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Level).To(Equal(slog.LevelError))
			Expect(recs[0].Message).To(Equal(lvlPanicMessage))
		})

		DescribeTable("carries the recovered value verbatim",
			func(value any, check func(logged slog.Value)) {
				lv := Level{lvlMember(ctrl, lvlStopPass, func(context.Context) {
					lvlErupt(value)
				})}

				lv.Stop(context.Background())

				recs := logs.records()
				Expect(recs).To(HaveLen(1))

				check(attrs(recs[0])["error"])
			},
			Entry("an error", errLvlBoom, func(logged slog.Value) {
				Expect(logged.Kind()).To(Equal(slog.KindAny))
				Expect(logged.Any()).To(BeIdenticalTo(errLvlBoom))
			}),
			Entry("a string", "lvl string panic", func(logged slog.Value) {
				Expect(logged.Kind()).To(Equal(slog.KindString))
				Expect(logged.String()).To(Equal("lvl string panic"))
			}),
		)

		It("captures a traceback naming the frame that panicked", func() {
			lv := Level{lvlMember(ctrl, lvlStartPass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})}

			Expect(lv.Start(context.Background())).To(MatchError(yama.ErrStartFailed))

			recs := logs.records()
			Expect(recs).To(HaveLen(1))

			stack := attrs(recs[0])["stack"]
			Expect(stack.Kind()).To(Equal(slog.KindString))
			Expect(stack.String()).To(HavePrefix("goroutine "))
			Expect(stack.String()).To(ContainSubstring("[running]:"))
			Expect(stack.String()).To(ContainSubstring("lvlErupt"))
		})

		It("writes to whichever slog default is installed when the pass runs", func() {
			lv := Level{lvlMember(ctrl, lvlStopPass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})}

			later := captureSlog()

			lv.Stop(context.Background())

			Expect(logs.records()).To(BeEmpty())
			Expect(later.records()).To(HaveLen(1))
		})
	})

	Context("when a member fails", func() {
		DescribeTable("leaves its siblings uncanceled and running",
			func(arrangeFailure func(m *execmocks.MockCompleteLifecycle, meet func())) {
				gate := lvlNewGate(2)
				met := make(chan bool, 2)
				rec := lvlNewRecorder()

				failing := execmocks.NewMockCompleteLifecycle(ctrl)
				arrangeFailure(failing, func() {
					met <- gate.arrive()
				})

				sibling := lvlMember(ctrl, lvlStartPass, func(ctx context.Context) {
					met <- gate.arrive()

					select {
					case <-ctx.Done():
						rec.note("canceled")
					case <-time.After(lvlGrace):
					}

					rec.note("finished")
				})

				ctx, cancel := context.WithCancel(context.Background())
				DeferCleanup(cancel)

				Expect(Level{failing, sibling}.Start(ctx)).To(MatchError(yama.ErrStartFailed))

				Expect(met).To(HaveLen(2))
				Expect(<-met).To(BeTrue())
				Expect(<-met).To(BeTrue())

				Expect(rec.count("canceled")).To(BeZero())
				Expect(rec.count("finished")).To(Equal(1))
			},
			Entry("by returning an error", func(m *execmocks.MockCompleteLifecycle, meet func()) {
				m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
					meet()

					return errLvlBoom
				})
			}),
			Entry("by panicking", func(m *execmocks.MockCompleteLifecycle, meet func()) {
				m.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
					meet()

					lvlErupt(errLvlBoom)

					return nil
				})
			}),
		)
	})

	Context("with a context that has already expired", func() {
		DescribeTable("still runs every member and reports nothing of its own",
			func(pass lvlPass) {
				rec := lvlNewRecorder()

				lv := lvlMembers(ctrl, pass, []string{"alpha", "bravo"},
					func(name string, _ context.Context) {
						rec.note(name)
					})

				pass.invoke(lv, expired())

				Expect(rec.counts()).To(Equal(map[string]int{"alpha": 1, "bravo": 1}))
				Expect(logs.records()).To(BeEmpty())
			},
			Entry("start", lvlStartPass),
			Entry("quiesce", lvlQuiescePass),
			Entry("stop", lvlStopPass),
		)

		It("decides Start on its members' outcomes alone", func() {
			succeeding := lvlMember(ctrl, lvlStartPass, func(context.Context) {})

			Expect(Level{succeeding}.Start(expired())).To(Succeed())

			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			failing.EXPECT().Start(gomock.Any()).Return(errLvlBoom)

			Expect(Level{failing}.Start(expired())).To(MatchError(yama.ErrStartFailed))
		})
	})

	Context("nested inside another Level", func() {
		It("reports an inner member's failure through the outer Start", func() {
			rec := lvlNewRecorder()

			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			failing.EXPECT().Start(gomock.Any()).Return(errLvlBoom)

			sibling := lvlMember(ctrl, lvlStartPass, func(context.Context) {
				rec.note("sibling")
			})

			Expect(Level{Level{failing}, sibling}.Start(context.Background())).
				To(MatchError(yama.ErrStartFailed))

			Expect(rec.count("sibling")).To(Equal(1))
		})

		It("contains an inner member's panic at both levels", func() {
			rec := lvlNewRecorder()

			panicking := lvlMember(ctrl, lvlQuiescePass, func(context.Context) {
				lvlErupt(errLvlBoom)
			})

			sibling := lvlMember(ctrl, lvlQuiescePass, func(context.Context) {
				time.Sleep(lvlSettle)

				rec.note("sibling")
			})

			outer := Level{Level{panicking}, sibling}

			Expect(func() {
				outer.Quiesce(context.Background())
			}).NotTo(Panic())

			Expect(rec.count("sibling")).To(Equal(1))
			Expect(logs.records()).To(HaveLen(1))
		})
	})
})
