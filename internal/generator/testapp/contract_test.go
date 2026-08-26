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

package testapp_test

import (
	"context"
	"errors"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/generator/testapp"
)

// The graph the generated constructor builds, level by level:
//
//	level 0: res (cleanup only)
//	level 1: base
//	level 2: mid, leaf (leaf carries a cleanup)
//	level 3: top
//
// A level's members run concurrently, so a spec that spans one level compares
// positions rather than a fixed sequence.

var _ = Describe("the generated lifecycle constructor", func() {
	var (
		ctx context.Context
		rec *testapp.Recorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		rec = testapp.NewRecorder()
	})

	// before is the position of an event in the recording, and fails the spec
	// when the event was never recorded.
	before := func(event string) int {
		GinkgoHelper()

		i := slices.Index(rec.Events(), event)
		Expect(i).NotTo(Equal(-1), "%s was never recorded in %v", event, rec.Events())

		return i
	}

	Context("construction", func() {
		It("returns the application beside a usable Lifecycle", func() {
			top, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})

			Expect(err).NotTo(HaveOccurred())
			Expect(top).NotTo(BeNil())
			Expect(lifecycle).NotTo(BeNil())
		})

		It("runs no cleanup of its own, so teardown reaches the graph only through Stop", func() {
			_, _, err := testapp.NewLifecycle(rec, testapp.Fault{})

			Expect(err).NotTo(HaveOccurred())
			Expect(rec.Matching("cleanup")).To(BeEmpty())
		})
	})

	Context("ordering", func() {
		It("finishes starting a level before the next level begins", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).To(Succeed())

			Expect(before("start base")).To(BeNumerically("<", before("start mid")))
			Expect(before("start base")).To(BeNumerically("<", before("start leaf")))
			Expect(before("start mid")).To(BeNumerically("<", before("start top")))
			Expect(before("start leaf")).To(BeNumerically("<", before("start top")))
		})

		It("runs both shutdown passes dependents-first, the reverse of startup", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)

			Expect(before("quiesce top")).To(BeNumerically("<", before("quiesce mid")))
			Expect(before("quiesce mid")).To(BeNumerically("<", before("quiesce base")))
			Expect(before("stop top")).To(BeNumerically("<", before("stop leaf")))
			Expect(before("stop leaf")).To(BeNumerically("<", before("stop base")))
		})

		It("finishes the whole quiesce pass before the teardown pass begins", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)

			Expect(before("quiesce base")).To(BeNumerically("<", before("stop top")))
		})
	})

	Context("cleanup", func() {
		It("runs every Google Wire cleanup exactly once, as part of Stop", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())
			Expect(rec.Matching("cleanup")).To(BeEmpty(), "a cleanup ran before Stop")

			lifecycle.Stop(ctx)

			Expect(rec.Matching("cleanup")).To(ConsistOf("cleanup leaf", "cleanup res"))
		})

		It("runs a repeated Stop without tearing anything down twice", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)
			lifecycle.Stop(ctx)

			Expect(rec.Matching("cleanup")).To(ConsistOf("cleanup leaf", "cleanup res"))
			Expect(rec.Matching("stop ")).To(ConsistOf("stop top", "stop mid", "stop leaf", "stop base"))
		})

		It("runs a component's cleanup ahead of that component's own Stop", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)

			Expect(before("cleanup leaf")).To(BeNumerically("<", before("stop leaf")))
		})

		It("runs a standalone cleanup at its own value's position in the ordering", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)

			Expect(before("stop base")).To(BeNumerically("<", before("cleanup res")))
		})
	})

	Context("startup failure", func() {
		It("stops the traversal at the failing level and reports ErrStartFailed", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{FailStart: "mid"})
			Expect(err).NotTo(HaveOccurred())

			startErr := lifecycle.Start(ctx)

			Expect(startErr).To(MatchError(yama.ErrStartFailed))
			Expect(errors.Is(startErr, testapp.ErrStartFault)).To(BeFalse(),
				"a component's own error must not reach the caller")
			Expect(rec.Events()).NotTo(ContainElement("start top"))
		})

		It("tears down everything it reached, cleanups included", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{FailStart: "mid"})
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).NotTo(Succeed())

			Expect(rec.Events()).To(ContainElement("stop base"))
			Expect(rec.Matching("cleanup")).To(ConsistOf("cleanup leaf", "cleanup res"))
		})

		It("runs the cleanups of the levels the failing start never reached", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{FailStart: "base"})
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).NotTo(Succeed())

			Expect(rec.Matching("cleanup")).To(ConsistOf("cleanup leaf", "cleanup res"))
			Expect(rec.Events()).NotTo(ContainElement("quiesce leaf"))
			Expect(rec.Events()).NotTo(ContainElement("stop leaf"))
			Expect(rec.Events()).NotTo(ContainElement(HaveSuffix(" mid")), "an unreached plain component receives no call")
			Expect(rec.Events()).NotTo(ContainElement(HaveSuffix(" top")), "an unreached plain component receives no call")
		})

		It("gates the component whose Start failed out of both shutdown passes", func() {
			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{FailStart: "mid"})
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).NotTo(Succeed())

			Expect(rec.Events()).NotTo(ContainElement("quiesce mid"))
			Expect(rec.Events()).NotTo(ContainElement("stop mid"))
		})
	})

	Context("options supplied at the call site", func() {
		It("applies an interceptor to every component implementing the operation", func() {
			observer := &identityInterceptor{rec: rec}

			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{},
				yama.WithInterceptors(observer))
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).To(Succeed())

			Expect(rec.Matching("intercept ")).To(HaveLen(len(rec.Matching("start "))))
		})

		It("opens with the begin boundary set and closes with the end boundary set", func() {
			begin := testapp.NewProbe(rec, "begin", testapp.Fault{})
			end := testapp.NewProbe(rec, "end", testapp.Fault{})

			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{},
				yama.WithBeginComponents(begin), yama.WithEndComponents(end))
			Expect(err).NotTo(HaveOccurred())
			Expect(lifecycle.Start(ctx)).To(Succeed())

			lifecycle.Stop(ctx)

			Expect(before("start begin")).To(BeNumerically("<", before("start base")))
			Expect(before("start top")).To(BeNumerically("<", before("start end")))
			Expect(before("stop end")).To(BeNumerically("<", before("stop top")))
			Expect(before("stop base")).To(BeNumerically("<", before("stop begin")))
		})
	})

	Context("component identity", func() {
		It("gives each component's chain that component's own instance", func() {
			observer := &identityInterceptor{rec: rec}

			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{},
				yama.WithInterceptors(observer))
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).To(Succeed())

			Expect(rec.Matching("intercept ")).To(ConsistOf(
				"intercept base", "intercept mid", "intercept leaf", "intercept top"))
		})

		It("tells two components of one type apart", func() {
			observer := &identityInterceptor{rec: rec}
			first := testapp.NewProbe(rec, "first", testapp.Fault{})
			second := testapp.NewProbe(rec, "second", testapp.Fault{})

			_, lifecycle, err := testapp.NewLifecycle(rec, testapp.Fault{},
				yama.WithInterceptors(observer),
				yama.WithBeginComponents(first, second))
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycle.Start(ctx)).To(Succeed())

			Expect(rec.Matching("intercept ")).To(ContainElements(
				"intercept first", "intercept second"))
		})
	})
})

// identityInterceptor records the component the context carries, so a spec can
// compare it against the component whose Start actually ran.
type identityInterceptor struct {
	rec *testapp.Recorder
}

func (i *identityInterceptor) Start(ctx context.Context, next yama.Starter) error {
	component, ok := yama.FromContext[interface{ String() string }](ctx)
	if !ok {
		i.rec.Record("intercept <none>")

		return next.Start(ctx)
	}
	i.rec.Record("intercept " + component.String())

	return next.Start(ctx)
}
