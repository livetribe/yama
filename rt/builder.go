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
	"l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/bridge"
	"l7e.io/yama/v2/rt/internal/exec"
)

// LifecycleBuilder assembles a Lifecycle one level at a time. Generated code adds
// levels in dependency order: the first level added starts first, and shutdown
// walks the levels in reverse. The begin boundary set is added as the first level
// by NewLifecycleBuilder and the end boundary set as the last by Build, so
// generated code adds only the graph's own levels between them.
//
// A builder is single-use. Every method panics once Build has been called.
type LifecycleBuilder struct {
	valid bool

	levels []exec.Level
	chains *exec.Chains
	end    []any
	built  bool
}

// NewLifecycleBuilder applies the construction options, builds the three
// interceptor chains once for reuse across every component, and opens the begin
// boundary set as the first level.
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

	b.NextLevel().
		WithComponents(begin...).
		Add()

	return b
}

// NextLevel opens the next level. Its members run concurrently with one another
// and after every member of the levels already added. The level joins the
// lifecycle when LevelBuilder.Add is called; a level that is never added is
// discarded.
func (b *LifecycleBuilder) NextLevel() *LevelBuilder {
	b.check()

	return newLevelBuilder(b.chains, func(level exec.Level) {
		b.levels = append(b.levels, level)
	})
}

// Build appends the end boundary set as the final level and returns the
// Lifecycle. Any further use of the builder panics.
func (b *LifecycleBuilder) Build() yama.Lifecycle {
	b.check()

	b.NextLevel().
		WithComponents(b.end...).
		Add()

	b.built = true
	return exec.NewLifecycle(b.levels)
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

// LevelBuilder accumulates the members of a single level. Order of accumulation
// carries no meaning: the members run concurrently.
//
// A LevelBuilder is single-use. Every method panics once Add has been called.
type LevelBuilder struct {
	valid bool

	chains     *exec.Chains
	components []exec.CompleteLifecycle

	done  func(level exec.Level)
	built bool
}

func newLevelBuilder(chains *exec.Chains, done func(level exec.Level)) *LevelBuilder {
	return &LevelBuilder{
		valid:  true,
		chains: chains,
		done:   done,
	}
}

// WithComponents adds components to the level, each bound through the interceptor
// chains. A component joins only the passes whose capability interfaces it
// implements; one implementing none is inert.
func (b *LevelBuilder) WithComponents(components ...any) *LevelBuilder {
	b.check()

	for _, c := range components {
		b.components = append(b.components, b.chains.WrapComponent(c))
	}

	return b
}

// WithComponentAndCleanup adds a component whose provider also returned a Google
// Wire cleanup function. The cleanup runs as part of that component's teardown,
// ahead of the component's own Stop, and does not pass through the interceptor
// chains.
func (b *LevelBuilder) WithComponentAndCleanup(c any, cleanup func()) *LevelBuilder {
	b.check()

	b.components = append(b.components, exec.NewCleanup(b.chains.WrapComponent(c), cleanup))

	return b
}

// Add seals the level and hands it to the lifecycle being built.
func (b *LevelBuilder) Add() {
	b.check()

	b.done(b.components)
	b.built = true
}

// check panics on reuse after Add. Misuse is a generation bug, not a runtime
// condition a caller can recover from.
func (b *LevelBuilder) check() {
	if !b.valid {
		panic("invalid level builder")
	}
	if b.built {
		panic("level already built and added")
	}
}
