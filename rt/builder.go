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

package rt

import (
	"l7e.io/yama"
	"l7e.io/yama/internal/bridge"
	"l7e.io/yama/rt/internal/exec"
)

// LifecycleBuilder assembles a Lifecycle one level at a time. NextLevel starts a
// new level, and every member added after it belongs to that level. NextLevel and
// the With… methods return the builder, so one expression can declare a whole
// lifecycle.
//
// Generated code adds levels in dependency order: the first level added starts
// first, and shutdown walks the levels in reverse. The begin boundary set has the
// first level to itself and the end boundary set has the last, so generated code
// adds only the graph's own levels between them. Generated code starts each of
// those levels with NextLevel, including the first.
//
// A builder is single-use. Every method panics once Build has been called.
type LifecycleBuilder struct {
	valid bool

	levels []exec.Level
	chains *exec.Chains
	end    []any

	current []exec.CompleteLifecycle

	built bool
}

// NewLifecycleBuilder applies the construction options, builds the three
// interceptor chains once for reuse across every component, and adds the begin
// boundary set as a first level of its own.
func NewLifecycleBuilder(opts ...yama.Option) *LifecycleBuilder {
	cfg := &bridge.Config{}
	for _, o := range opts {
		o.Apply(cfg)
	}

	chains := exec.NewChains(cfg.Interceptors)
	begin := cfg.BeginComponents
	end := cfg.EndComponents

	b := &LifecycleBuilder{
		valid:  true,
		chains: chains,
		end:    end,
	}

	b.WithComponents(begin...)

	return b
}

// NextLevel starts the next level. Every member added after it belongs to that
// level. Those members run concurrently with one another, and after every member
// of the levels before them. A level with no members is not added.
func (b *LifecycleBuilder) NextLevel() *LifecycleBuilder {
	b.check()

	b.flushCurrentLevel()

	return b
}

// WithComponents adds components to the current level, each bound through the
// interceptor chains. A component joins only the passes whose capability
// interfaces it implements; one implementing none is inert.
func (b *LifecycleBuilder) WithComponents(components ...any) *LifecycleBuilder {
	b.check()

	for _, c := range components {
		b.current = append(b.current, b.chains.WrapComponent(c))
	}

	return b
}

// WithCleanableComponent adds a component to the current level. The component's
// provider also returned a Google Wire cleanup function. The cleanup runs as part
// of that component's teardown, ahead of the component's own Stop, and does not
// pass through the interceptor chains.
func (b *LifecycleBuilder) WithCleanableComponent(c any, cleanup func()) *LifecycleBuilder {
	b.check()

	b.current = append(b.current, exec.NewCleanableComponent(b.chains.WrapComponent(c), cleanup))

	return b
}

// WithCleanup adds a Google Wire cleanup function to the current level. The value
// it releases implements no lifecycle capability of its own. The cleanup is the
// value's whole teardown, runs at the value's position in the ordering, and does
// not pass through the interceptor chains.
func (b *LifecycleBuilder) WithCleanup(cleanup func()) *LifecycleBuilder {
	b.check()

	b.current = append(b.current, exec.Cleanup(cleanup))

	return b
}

// Build adds the end boundary set as a final level of its own and returns the
// Lifecycle. Any further use of the builder panics.
func (b *LifecycleBuilder) Build() yama.Lifecycle {
	b.check()

	b.flushCurrentLevel()
	b.WithComponents(b.end...)
	b.flushCurrentLevel()

	b.built = true

	return exec.NewLifecycle(b.levels)
}

// flushCurrentLevel moves the members of the current level into the level list.
// It ignores a level that has no members.
func (b *LifecycleBuilder) flushCurrentLevel() {
	if b.current == nil {
		return
	}

	b.levels = append(b.levels, b.current)
	b.current = nil
}

// check panics on reuse after Build. Misuse is a generation bug, not a runtime
// condition a caller can recover from.
func (b *LifecycleBuilder) check() {
	if !b.valid {
		panic("invalid lifecycle builder")
	}
	if b.built {
		panic("lifecycle already built")
	}
}
