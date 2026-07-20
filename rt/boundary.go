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
	"context"

	yama "l7e.io/yama/v2"
)

// Boundary runs one flat, unordered set of boundary components — a begin set or
// an end set — through a lifecycle pass. A boundary component participates in a
// pass through whichever of Starter, Quiescer, and Stopper it implements, wrapped
// through the same chains as a graph component, so its failure handling is
// identical. The set has no internal ordering; its members run concurrently.
type Boundary struct {
	components []*boundaryComponent
}

// boundaryComponent holds one boundary component's wrapped operations and its
// started flag. Each wrapped field is nil when the component does not implement
// that capability. Every component carries a started flag: a Starter's is set when
// its Start succeeds, a non-Starter's is marked when the boundary is reached, so
// quiesce and teardown gate on the same signal a graph component's do.
type boundaryComponent struct {
	start   yama.Starter
	quiesce yama.Quiescer
	stop    yama.Stopper
	started Flag
}

// newBoundary wraps each component through chains for every capability it
// implements.
func newBoundary(components []any, chains *Chains) *Boundary {
	b := &Boundary{}

	for _, comp := range components {
		b.components = append(b.components, &boundaryComponent{
			start:   chains.WrapStart(comp),
			quiesce: chains.WrapQuiesce(comp),
			stop:    chains.WrapStop(comp),
		})
	}

	return b
}

// Start runs the set's Starters concurrently with fail-fast, marking each that
// succeeds as started; a non-Starter is marked started because the boundary was
// reached. It returns the first failure, or nil.
func (b *Boundary) Start(ctx context.Context) error {
	g := NewGroup(ctx)
	for _, c := range b.components {
		if c.start != nil {
			g.Go(c.start, &c.started)
		} else {
			g.Go(Started, &c.started)
		}
	}

	return g.Wait()
}

// Quiesce runs the started Quiescers concurrently, swallowing panics, and waits
// for all to return.
func (b *Boundary) Quiesce(ctx context.Context) {
	g := NewShutdownGroup(ctx)
	for _, c := range b.components {
		if c.quiesce != nil {
			g.Go(c.quiesce.Quiesce, &c.started)
		}
	}

	g.Wait()
}

// Stop runs the started Stoppers concurrently, swallowing panics, and waits for
// all to return.
func (b *Boundary) Stop(ctx context.Context) {
	g := NewShutdownGroup(ctx)
	for _, c := range b.components {
		if c.stop != nil {
			g.Go(c.stop.Stop, &c.started)
		}
	}

	g.Wait()
}
