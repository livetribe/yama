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

package graph_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
)

// names returns the name of every member of every level, level by level. Most
// specs assert on this rather than on whole Member values, because the question
// they ask is which level a component landed in.
func names(levels [][]graph.Member) [][]string {
	out := make([][]string, len(levels))

	for i, level := range levels {
		for _, m := range level {
			out[i] = append(out[i], m.Name)
		}
	}

	return out
}

// lifecycle returns a component that declares every lifecycle method.
func lifecycle(name string, deps ...string) graph.Component {
	return graph.Component{
		Name:         name,
		Deps:         deps,
		Capabilities: graph.Start | graph.Quiesce | graph.Stop,
	}
}

// plain returns a component that declares no lifecycle method and returned no
// cleanup, so it occupies no level of its own.
func plain(name string, deps ...string) graph.Component {
	return graph.Component{Name: name, Deps: deps}
}

var _ = Describe("graph", func() {
	Describe("Capability", func() {
		It("reports a capability that the set holds", func() {
			Expect((graph.Start | graph.Stop).Has(graph.Start)).To(BeTrue())
		})

		It("reports a capability that the set does not hold", func() {
			Expect((graph.Start | graph.Stop).Has(graph.Quiesce)).To(BeFalse())
		})

		It("names an empty set", func() {
			Expect(graph.None.String()).To(Equal("none"))
		})

		It("names the capabilities in a fixed order", func() {
			Expect((graph.Stop | graph.Start).String()).To(Equal("start|stop"))
		})
	})

	Describe("Levels", func() {
		Context("when no component consumes another", func() {
			It("puts every component in the first level", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("a"),
					lifecycle("b"),
					lifecycle("c"),
				})

				Expect(names(levels)).To(Equal([][]string{{"a", "b", "c"}}))
			})

			It("keeps the order that Google Wire emitted", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("c"),
					lifecycle("a"),
					lifecycle("b"),
				})

				Expect(names(levels)).To(Equal([][]string{{"c", "a", "b"}}))
			})
		})

		Context("when one component consumes another", func() {
			It("puts the consumer one level behind", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					lifecycle("repo", "db"),
					lifecycle("server", "repo"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}, {"repo"}, {"server"}}))
			})

			It("puts a component behind the deepest thing it consumes", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					lifecycle("cache"),
					lifecycle("repo", "db"),
					lifecycle("server", "cache", "repo"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db", "cache"}, {"repo"}, {"server"}}))
			})

			It("puts two consumers of one component in the same level", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					lifecycle("users", "db"),
					lifecycle("orders", "db"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}, {"users", "orders"}}))
			})

			It("rejoins two paths that split and meet again", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					lifecycle("users", "db"),
					lifecycle("orders", "db"),
					lifecycle("api", "users", "orders"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}, {"users", "orders"}, {"api"}}))
			})
		})

		Context("when a component occupies no level", func() {
			It("leaves it out of every level", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					plain("config"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}}))
			})

			It("carries ordering across it", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					plain("dsn", "db"),
					lifecycle("repo", "dsn"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}, {"repo"}}))
			})

			It("carries ordering across a run of them", func() {
				levels := graph.Levels([]graph.Component{
					lifecycle("db"),
					plain("one", "db"),
					plain("two", "one"),
					lifecycle("repo", "two"),
				})

				Expect(names(levels)).To(Equal([][]string{{"db"}, {"repo"}}))
			})

			It("returns no level for a package whose components all pass through", func() {
				levels := graph.Levels([]graph.Component{
					plain("config"),
					plain("dsn", "config"),
				})

				Expect(levels).To(BeEmpty())
			})
		})

		Context("when a provider returned a cleanup", func() {
			It("gives the component a level of its own", func() {
				levels := graph.Levels([]graph.Component{
					{Name: "tmpdir", Cleanup: "cleanup"},
				})

				Expect(names(levels)).To(Equal([][]string{{"tmpdir"}}))
			})

			It("carries the cleanup onto the member", func() {
				levels := graph.Levels([]graph.Component{
					{Name: "tmpdir", Cleanup: "cleanup"},
				})

				Expect(levels[0][0].Cleanup).To(Equal("cleanup"))
			})

			It("puts a lifecycle component that consumes it one level behind", func() {
				levels := graph.Levels([]graph.Component{
					{Name: "tmpdir", Cleanup: "cleanup"},
					lifecycle("store", "tmpdir"),
				})

				Expect(names(levels)).To(Equal([][]string{{"tmpdir"}, {"store"}}))
			})
		})

		Context("what a member carries", func() {
			It("carries the component's capabilities", func() {
				levels := graph.Levels([]graph.Component{
					{Name: "db", Capabilities: graph.Start | graph.Stop},
				})

				Expect(levels[0][0].Capabilities).To(Equal(graph.Start | graph.Stop))
			})
		})

		It("returns no level for an injector that builds nothing", func() {
			Expect(graph.Levels(nil)).To(BeEmpty())
		})
	})
})
