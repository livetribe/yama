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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/mocks"
	"l7e.io/yama/v2/rt"
	execmocks "l7e.io/yama/v2/rt/internal/mocks"
)

// A call a spec never sets an expectation for fails the moment it happens, so
// "must not be quiesced" and "must never start" are expressed by silence rather
// than by asserting a zero count afterward.

// contractBound caps a rendezvous so a sequential regression fails fast instead
// of hanging.
const contractBound = time.Second

var _ = Describe("the rt lifecycle contract", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	Context("ordering", func() {
		It("finishes starting a level before the next level begins", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			mid := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			gomock.InOrder(
				base.EXPECT().Start(gomock.Any()),
				mid.EXPECT().Start(gomock.Any()),
				top.EXPECT().Start(gomock.Any()),
			)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(mid)
			b.NextLevel().WithComponents(top)

			Expect(b.Build().Start(context.Background())).To(Succeed())
		})

		DescribeTable("runs both shutdown passes dependents-first, the reverse of startup",
			func(ordered func(m *execmocks.MockCompleteLifecycle) *gomock.Call, other func(m *execmocks.MockCompleteLifecycle)) {
				base := execmocks.NewMockCompleteLifecycle(ctrl)
				mid := execmocks.NewMockCompleteLifecycle(ctrl)
				top := execmocks.NewMockCompleteLifecycle(ctrl)

				for _, c := range []*execmocks.MockCompleteLifecycle{base, mid, top} {
					c.EXPECT().Start(gomock.Any())
					other(c)
				}
				gomock.InOrder(ordered(top), ordered(mid), ordered(base))

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(base)
				b.NextLevel().WithComponents(mid)
				b.NextLevel().WithComponents(top)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())
				lc.Stop(context.Background())
			},
			Entry("quiesce",
				func(m *execmocks.MockCompleteLifecycle) *gomock.Call {
					return m.EXPECT().Quiesce(gomock.Any())
				},
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Stop(gomock.Any())
				}),
			Entry("stop",
				func(m *execmocks.MockCompleteLifecycle) *gomock.Call {
					return m.EXPECT().Stop(gomock.Any())
				},
				func(m *execmocks.MockCompleteLifecycle) {
					m.EXPECT().Quiesce(gomock.Any())
				}),
		)

		It("completes every quiesce before every teardown, without constraining order within either pass", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			mid := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)
			all := []*execmocks.MockCompleteLifecycle{base, mid, top}

			quiesces := make([]*gomock.Call, 0, len(all))
			for _, c := range all {
				c.EXPECT().Start(gomock.Any())
				quiesces = append(quiesces, c.EXPECT().Quiesce(gomock.Any()))
			}
			for _, c := range all {
				stop := c.EXPECT().Stop(gomock.Any())
				for _, q := range quiesces {
					stop.After(q)
				}
			}

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(mid)
			b.NextLevel().WithComponents(top)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())
		})

		It("keeps two quiescing components ordered across a level that implements no Quiescer", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			mid := mocks.NewMockLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			// mid is a Lifecycle — Starter and Stopper only — so it never joins
			// the quiesce pass by type, not by any expectation set here.
			base.EXPECT().Start(gomock.Any())
			mid.EXPECT().Start(gomock.Any())
			top.EXPECT().Start(gomock.Any())
			base.EXPECT().Stop(gomock.Any())
			mid.EXPECT().Stop(gomock.Any())
			top.EXPECT().Stop(gomock.Any())
			gomock.InOrder(
				top.EXPECT().Quiesce(gomock.Any()),
				base.EXPECT().Quiesce(gomock.Any()),
			)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(mid)
			b.NextLevel().WithComponents(top)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())
		})

		It("tears down a folded cleanup at its own value's position, dependents first", func() {
			var events []string

			base := execmocks.NewMockCompleteLifecycle(ctrl)
			mid := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			record := func(name string, c *execmocks.MockCompleteLifecycle) {
				c.EXPECT().Start(gomock.Any())
				c.EXPECT().Quiesce(gomock.Any())
				c.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
					events = append(events, name+".Stop")
				})
			}
			record("base", base)
			record("mid", mid)
			record("top", top)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithCleanableComponent(base, func() {
				events = append(events, "base.cleanup")
			})
			b.NextLevel().WithComponents(mid)
			b.NextLevel().WithCleanableComponent(top, func() {
				events = append(events, "top.cleanup")
			})
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())

			// The deepest cleanup runs last of all, not first: a cleanup inherits
			// the ordering of the value it cleans up rather than Wire's own
			// aggregated teardown order.
			Expect(events).To(Equal([]string{
				"top.cleanup", "top.Stop",
				"mid.Stop",
				"base.cleanup", "base.Stop",
			}))
		})

		It("tears down a level's plain and cleanup-bearing members together", func() {
			var mu sync.Mutex
			var events []string

			record := func(s string) {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, s)
			}

			worker := execmocks.NewMockCompleteLifecycle(ctrl)
			conn := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			for _, c := range []*execmocks.MockCompleteLifecycle{worker, conn, top} {
				c.EXPECT().Start(gomock.Any())
				c.EXPECT().Quiesce(gomock.Any())
			}

			cleaned := false
			connStopSawCleanup := false
			worker.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				record("worker.Stop")
			})
			conn.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				connStopSawCleanup = cleaned
				record("conn.Stop")
			})
			top.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				record("top.Stop")
			})

			b := rt.NewLifecycleBuilder()
			b.NextLevel().
				WithComponents(worker).
				WithCleanableComponent(conn, func() {
					cleaned = true
					record("conn.cleanup")
				})
			b.NextLevel().WithComponents(top)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())

			// Two teardown participants share the level, so the order between them
			// is unconstrained; both still follow the level that depends on them.
			Expect(events).To(HaveLen(4))
			Expect(events[0]).To(Equal("top.Stop"))
			Expect(events[1:]).To(ConsistOf("worker.Stop", "conn.cleanup", "conn.Stop"))
			Expect(connStopSawCleanup).To(BeTrue(), "the cleanup must run before its own component's Stop")
		})

		It("completes every quiesce before any folded cleanup runs", func() {
			quiesced := 0
			standaloneRuns, standaloneSaw := 0, -1
			pairedRuns, pairedSaw := 0, -1

			conn := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			for _, c := range []*execmocks.MockCompleteLifecycle{conn, top} {
				c.EXPECT().Start(gomock.Any())
				c.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
					quiesced++
				})
				c.EXPECT().Stop(gomock.Any())
			}

			b := rt.NewLifecycleBuilder()
			b.NextLevel().
				WithCleanup(func() {
					standaloneRuns++
					standaloneSaw = quiesced
				}).
				WithCleanableComponent(conn, func() {
					pairedRuns++
					pairedSaw = quiesced
				})
			b.NextLevel().WithComponents(top)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())

			// A cleanup releases what a still-quiescing component may need, so it
			// belongs wholly inside the teardown pass in either of its forms: it
			// runs once, and only after both components have quiesced.
			Expect(standaloneRuns).To(Equal(1))
			Expect(pairedRuns).To(Equal(1))
			Expect(standaloneSaw).To(Equal(2))
			Expect(pairedSaw).To(Equal(2))
		})

		It("starts a level's members at once", func() {
			verifyNoLeaks()

			var wg sync.WaitGroup
			wg.Add(2)
			arrive := func(context.Context) error {
				wg.Done()
				wg.Wait()
				return nil
			}

			router := execmocks.NewMockCompleteLifecycle(ctrl)
			worker := execmocks.NewMockCompleteLifecycle(ctrl)
			router.EXPECT().Start(gomock.Any()).DoAndReturn(arrive)
			worker.EXPECT().Start(gomock.Any()).DoAndReturn(arrive)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(router, worker)

			started := make(chan error, 1)
			go func() {
				started <- b.Build().Start(context.Background())
			}()

			Eventually(started).WithTimeout(contractBound).Should(Receive(BeNil()))
		})

		DescribeTable("runs a level's members at once in the quiesce and teardown passes",
			func(expect func(m *execmocks.MockCompleteLifecycle, arrive func(context.Context))) {
				var wg sync.WaitGroup
				wg.Add(2)
				arrive := func(context.Context) {
					wg.Done()
					wg.Wait()
				}

				router := execmocks.NewMockCompleteLifecycle(ctrl)
				worker := execmocks.NewMockCompleteLifecycle(ctrl)
				for _, m := range []*execmocks.MockCompleteLifecycle{router, worker} {
					m.EXPECT().Start(gomock.Any())
					expect(m, arrive)
				}

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(router, worker)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())

				stopped := make(chan struct{})
				go func() {
					lc.Stop(context.Background())
					close(stopped)
				}()

				Eventually(stopped).WithTimeout(contractBound).Should(BeClosed())
			},
			Entry("quiesce", func(m *execmocks.MockCompleteLifecycle, arrive func(context.Context)) {
				m.EXPECT().Quiesce(gomock.Any()).Do(arrive)
				m.EXPECT().Stop(gomock.Any())
			}),
			Entry("stop", func(m *execmocks.MockCompleteLifecycle, arrive func(context.Context)) {
				m.EXPECT().Quiesce(gomock.Any())
				m.EXPECT().Stop(gomock.Any()).Do(arrive)
			}),
		)

		It("delivers callbacks to a component only for the capabilities its type declares", func() {
			starter := mocks.NewMockStarter(ctrl)
			stopper := mocks.NewMockStopper(ctrl)

			starter.EXPECT().Start(gomock.Any())
			stopper.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(starter, stopper)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())
		})
	})

	Context("startup failure", func() {
		It("stops the traversal at a failed level, leaving later levels untouched", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			mid := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)

			base.EXPECT().Start(gomock.Any())
			mid.EXPECT().Start(gomock.Any()).Return(errors.New("mid boom"))
			base.EXPECT().Quiesce(gomock.Any())
			base.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(mid)
			b.NextLevel().WithComponents(top)

			Expect(b.Build().Start(context.Background())).To(MatchError(yama.ErrStartFailed))
		})

		DescribeTable("scopes cleanup to components that came up",
			func(fail func(c *gomock.Call)) {
				sibling := execmocks.NewMockCompleteLifecycle(ctrl)
				failing := execmocks.NewMockCompleteLifecycle(ctrl)

				sibling.EXPECT().Start(gomock.Any())
				sibling.EXPECT().Quiesce(gomock.Any())
				sibling.EXPECT().Stop(gomock.Any())
				fail(failing.EXPECT().Start(gomock.Any()))

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(sibling, failing)
				lc := b.Build()

				var err error
				Expect(func() {
					err = lc.Start(context.Background())
				}).NotTo(Panic())
				Expect(err).To(MatchError(yama.ErrStartFailed))
			},
			Entry("start returns an error", func(c *gomock.Call) {
				c.Return(errors.New("boom"))
			}),
			Entry("start panics", func(c *gomock.Call) {
				c.DoAndReturn(func(context.Context) error {
					panic("boom")
				})
			}),
		)

		It("recovers a panicking Start interceptor as a start failure, cleaning up what came up", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)
			interceptor := mocks.NewMockStartInterceptor(ctrl)

			// The interceptor is global: it passes base through, then panics ahead
			// of failing's own Start, which is never reached. failing sets no
			// expectations, so starting or tearing it down would fail the spec.
			base.EXPECT().Start(gomock.Any())
			base.EXPECT().Quiesce(gomock.Any())
			base.EXPECT().Stop(gomock.Any())
			gomock.InOrder(
				interceptor.EXPECT().Start(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, next yama.Starter) error {
						return next.Start(ctx)
					}),
				interceptor.EXPECT().Start(gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, yama.Starter) error {
						panic("interceptor boom")
					}),
			)

			b := rt.NewLifecycleBuilder(yama.WithInterceptors(interceptor))
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(failing)

			var err error
			Expect(func() {
				err = b.Build().Start(context.Background())
			}).NotTo(Panic())
			Expect(err).To(MatchError(yama.ErrStartFailed))
		})

		It("does not interrupt the components running beside a failure", func() {
			verifyNoLeaks()

			var wg sync.WaitGroup
			wg.Add(2)

			sibling := execmocks.NewMockCompleteLifecycle(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)

			sibling.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				wg.Done()
				wg.Wait()
				return nil
			})
			failing.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				wg.Done()
				wg.Wait()
				return errors.New("boom")
			})
			sibling.EXPECT().Quiesce(gomock.Any())
			sibling.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(sibling, failing)

			started := make(chan error, 1)
			go func() {
				started <- b.Build().Start(context.Background())
			}()

			Eventually(started).WithTimeout(contractBound).Should(Receive(MatchError(yama.ErrStartFailed)))
		})

		It("leaves a still-running sibling's context uncanceled when a member fails", func() {
			verifyNoLeaks()

			sibling := execmocks.NewMockCompleteLifecycle(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)

			// The sibling outlives the failure, then reports whether its own
			// context was canceled as a consequence. An implementation that
			// cancels siblings on failure trips ctx.Done here; the contract one
			// leaves it open, so the sibling waits out the grace and reports
			// nothing. Mocks ignore ctx, so this is the only pass that observes
			// cancellation at all — the rendezvous specs above cannot.
			failed := make(chan struct{})
			sawCancel := make(chan struct{})

			failing.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				close(failed)
				return errors.New("boom")
			})
			sibling.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
				<-failed
				select {
				case <-ctx.Done():
					close(sawCancel)
				case <-time.After(200 * time.Millisecond):
				}
				return nil
			})
			sibling.EXPECT().Quiesce(gomock.Any())
			sibling.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(failing, sibling)

			started := make(chan error, 1)
			go func() {
				started <- b.Build().Start(context.Background())
			}()

			Eventually(started).WithTimeout(contractBound).Should(Receive(MatchError(yama.ErrStartFailed)))
			Expect(sawCancel).NotTo(BeClosed())
		})

		It("returns ErrStartFailed and never the component's own error", func() {
			sentinel := errors.New("component-specific failure")

			only := execmocks.NewMockCompleteLifecycle(ctrl)
			only.EXPECT().Start(gomock.Any()).Return(sentinel)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(only)

			err := b.Build().Start(context.Background())
			Expect(err).To(MatchError(yama.ErrStartFailed))
			Expect(err).NotTo(MatchError(sentinel))
		})

		It("cleans up a failed startup as the ordinary ordered shutdown, not a parallel path", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			top := execmocks.NewMockCompleteLifecycle(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)

			base.EXPECT().Start(gomock.Any())
			top.EXPECT().Start(gomock.Any())
			failing.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

			topQuiesce := top.EXPECT().Quiesce(gomock.Any())
			baseQuiesce := base.EXPECT().Quiesce(gomock.Any()).After(topQuiesce)
			topStop := top.EXPECT().Stop(gomock.Any()).After(baseQuiesce)
			base.EXPECT().Stop(gomock.Any()).After(topStop)

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(top, failing)

			Expect(b.Build().Start(context.Background())).To(MatchError(yama.ErrStartFailed))
		})

		It("actually starts the components on a nil return, never a silent no-op", func() {
			first := execmocks.NewMockCompleteLifecycle(ctrl)
			second := execmocks.NewMockCompleteLifecycle(ctrl)

			first.EXPECT().Start(gomock.Any())
			second.EXPECT().Start(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(first)
			b.NextLevel().WithComponents(second)

			Expect(b.Build().Start(context.Background())).To(Succeed())
		})
	})

	Context("panics", func() {
		DescribeTable("recovers a panic in either shutdown pass and still tears down the deeper level",
			func(expect func(m *execmocks.MockCompleteLifecycle)) {
				base := execmocks.NewMockCompleteLifecycle(ctrl)
				top := execmocks.NewMockCompleteLifecycle(ctrl)

				base.EXPECT().Start(gomock.Any())
				top.EXPECT().Start(gomock.Any())
				base.EXPECT().Quiesce(gomock.Any())
				base.EXPECT().Stop(gomock.Any())
				expect(top)

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(base)
				b.NextLevel().WithComponents(top)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())
				Expect(func() {
					lc.Stop(context.Background())
				}).NotTo(Panic())
			},
			Entry("quiesce", func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
					panic("quiesce boom")
				})
				m.EXPECT().Stop(gomock.Any())
			}),
			Entry("stop", func(m *execmocks.MockCompleteLifecycle) {
				m.EXPECT().Quiesce(gomock.Any())
				m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
					panic("stop boom")
				})
			}),
		)
	})

	Context("boundaries obey the graph panic policy", func() {
		panicInQuiesce := func(m *execmocks.MockCompleteLifecycle) {
			m.EXPECT().Quiesce(gomock.Any()).Do(func(context.Context) {
				panic("boundary quiesce boom")
			})
			m.EXPECT().Stop(gomock.Any())
		}
		panicInStop := func(m *execmocks.MockCompleteLifecycle) {
			m.EXPECT().Quiesce(gomock.Any())
			m.EXPECT().Stop(gomock.Any()).Do(func(context.Context) {
				panic("boundary stop boom")
			})
		}

		It("fails startup when a begin boundary's Start panics, leaving the graph unstarted", func() {
			begin := execmocks.NewMockCompleteLifecycle(ctrl)
			graph := execmocks.NewMockCompleteLifecycle(ctrl)

			// graph sets no expectations: a begin failure must stop it starting.
			begin.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				panic("begin boom")
			})

			b := rt.NewLifecycleBuilder(yama.WithBeginComponents(begin))
			b.NextLevel().WithComponents(graph)

			var err error
			Expect(func() {
				err = b.Build().Start(context.Background())
			}).NotTo(Panic())
			Expect(err).To(MatchError(yama.ErrStartFailed))
		})

		It("fails startup and tears the graph down when an end boundary's Start panics", func() {
			end := execmocks.NewMockCompleteLifecycle(ctrl)
			graph := execmocks.NewMockCompleteLifecycle(ctrl)

			graph.EXPECT().Start(gomock.Any())
			end.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
				panic("end boom")
			})
			graph.EXPECT().Quiesce(gomock.Any())
			graph.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder(yama.WithEndComponents(end))
			b.NextLevel().WithComponents(graph)

			var err error
			Expect(func() {
				err = b.Build().Start(context.Background())
			}).NotTo(Panic())
			Expect(err).To(MatchError(yama.ErrStartFailed))
		})

		DescribeTable("swallows a boundary's shutdown-pass panic and still tears the graph down",
			func(atBegin bool, boundaryPanics func(m *execmocks.MockCompleteLifecycle)) {
				boundary := execmocks.NewMockCompleteLifecycle(ctrl)
				graph := execmocks.NewMockCompleteLifecycle(ctrl)

				boundary.EXPECT().Start(gomock.Any())
				graph.EXPECT().Start(gomock.Any())
				graph.EXPECT().Quiesce(gomock.Any())
				graph.EXPECT().Stop(gomock.Any())
				boundaryPanics(boundary)

				opt := yama.WithBeginComponents(boundary)
				if !atBegin {
					opt = yama.WithEndComponents(boundary)
				}

				b := rt.NewLifecycleBuilder(opt)
				b.NextLevel().WithComponents(graph)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())
				Expect(func() {
					lc.Stop(context.Background())
				}).NotTo(Panic())
			},
			Entry("begin boundary, panicking in quiesce", true, panicInQuiesce),
			Entry("begin boundary, panicking in stop", true, panicInStop),
			Entry("end boundary, panicking in quiesce", false, panicInQuiesce),
			Entry("end boundary, panicking in stop", false, panicInStop),
		)
	})

	Context("lifecycle semantics", func() {
		Describe("Stop is idempotent", func() {
			It("runs the passes once however many times Stop is called", func() {
				only := execmocks.NewMockCompleteLifecycle(ctrl)
				only.EXPECT().Start(gomock.Any())
				only.EXPECT().Quiesce(gomock.Any())
				only.EXPECT().Stop(gomock.Any())

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(only)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())
				lc.Stop(context.Background())
				lc.Stop(context.Background())
				lc.Stop(context.Background())
			})

			It("runs the passes once under concurrent Stop", func() {
				verifyNoLeaks()

				only := execmocks.NewMockCompleteLifecycle(ctrl)
				only.EXPECT().Start(gomock.Any())
				only.EXPECT().Quiesce(gomock.Any())
				only.EXPECT().Stop(gomock.Any())

				b := rt.NewLifecycleBuilder()
				b.NextLevel().WithComponents(only)
				lc := b.Build()

				Expect(lc.Start(context.Background())).To(Succeed())

				var wg sync.WaitGroup
				const callers = 32
				wg.Add(callers)
				for range callers {
					go func() {
						defer wg.Done()
						lc.Stop(context.Background())
					}()
				}

				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()

				Eventually(done).WithTimeout(contractBound).Should(BeClosed())
			})
		})

		It("does nothing when Stop follows a startup failure, cleanup having already run", func() {
			base := execmocks.NewMockCompleteLifecycle(ctrl)
			failing := execmocks.NewMockCompleteLifecycle(ctrl)

			base.EXPECT().Start(gomock.Any())
			base.EXPECT().Quiesce(gomock.Any())
			base.EXPECT().Stop(gomock.Any())
			failing.EXPECT().Start(gomock.Any()).Return(errors.New("boom"))

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(base)
			b.NextLevel().WithComponents(failing)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(MatchError(yama.ErrStartFailed))
			lc.Stop(context.Background())
		})

		It("restarts by re-running the passes, with no semantics beyond that", func() {
			only := execmocks.NewMockCompleteLifecycle(ctrl)

			only.EXPECT().Start(gomock.Any()).Times(2)
			only.EXPECT().Quiesce(gomock.Any())
			only.EXPECT().Stop(gomock.Any())

			b := rt.NewLifecycleBuilder()
			b.NextLevel().WithComponents(only)
			lc := b.Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			lc.Stop(context.Background())
			Expect(lc.Start(context.Background())).To(Succeed())
		})

		It("starts and stops an empty graph cleanly", func() {
			lc := rt.NewLifecycleBuilder().Build()

			Expect(lc.Start(context.Background())).To(Succeed())
			Expect(func() {
				lc.Stop(context.Background())
			}).NotTo(Panic())
		})
	})
})
