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

package generator

import (
	"fmt"
	"strings"
)

// Capabilities is the set of lifecycle capability interfaces a component's type
// implements, decided by static type analysis of the generated package.
type Capabilities uint8

const (
	// None is the empty set: a component with a type that implements none of the
	// three interfaces. It is a value and not merely an absence. Such a
	// component still occupies a level when its provider returned a cleanup
	// function.
	None Capabilities = 0
	// CanStart is yama.Starter.
	CanStart Capabilities = 1
	// CanQuiesce is yama.Quiescer.
	CanQuiesce Capabilities = 2
	// CanStop is yama.Stopper.
	CanStop Capabilities = 4
)

// Has reports whether every capability in other is present. Every set contains
// None, so test for the empty set by comparing against it rather than by asking.
func (c Capabilities) Has(other Capabilities) bool {
	return c&other == other
}

// String names the capabilities in lifecycle order, space separated, and is empty
// for None.
func (c Capabilities) String() string {
	var names []string
	if c.Has(CanStart) {
		names = append(names, "start")
	}
	if c.Has(CanQuiesce) {
		names = append(names, "quiesce")
	}
	if c.Has(CanStop) {
		names = append(names, "stop")
	}

	return strings.Join(names, " ")
}

// MemberKind is how a level member is handed to the runtime's level builder. A
// Google Wire cleanup function is not a member of its own: it is folded into the
// member that carries the value that it cleans up, which is what these three
// kinds distinguish.
type MemberKind int

const (
	// MemberComponent is the value alone, added with WithComponents. Its provider
	// returned no cleanup.
	MemberComponent MemberKind = iota
	// MemberCleanableComponent is the value paired with the cleanup that its
	// provider returned, added with WithCleanableComponent. The cleanup runs
	// ahead of the value's own Stop, and the pair is one teardown participant
	// rather than two.
	MemberCleanableComponent
	// MemberCleanup is the cleanup alone, added with WithCleanup. It is the whole
	// teardown of a value that implements no lifecycle capability.
	MemberCleanup
)

func (k MemberKind) String() string {
	switch k {
	case MemberComponent:
		return "component"
	case MemberCleanableComponent:
		return "cleanableComponent"
	case MemberCleanup:
		return "cleanup"
	default:
		return fmt.Sprintf("MemberKind(%d)", int(k))
	}
}

// Member is one component's place in a level: the component together with
// the capabilities that its type implements. A member joins only the passes
// for a capability that it has. One with no capability at all occupies its
// level for its cleanup alone.
type Member struct {
	Component    *Component
	Capabilities Capabilities
}

// Kind reports how the member is handed to the level builder. A value with a
// provider that returned no cleanup is added as a component. One that did is
// added as a cleanable component, or as the cleanup alone when the value
// implements no capability of its own.
//
// Every member falls in one of the three: a component occupies a level only for
// its capabilities, its cleanup, or both. Kind panics on the zero Member: a
// caller must check InjectorAnalysis.Member's ok result before calling it.
func (m Member) Kind() MemberKind {
	if m.Component == nil {
		panic("generator: Kind called on a Member with no Component")
	}

	if m.Component.Cleanup == nil {
		return MemberComponent
	}

	if m.Capabilities == None {
		return MemberCleanup
	}

	return MemberCleanableComponent
}

// Level is one group of components that have no lifecycle ordering constraint
// between them and can run concurrently. Members are in the order that their
// components were created in the injector body.
type Level []Member

// InjectorAnalysis is one injector's lifecycle analysis: the parsed graph and the
// single dependency-ordered level list computed over it. Startup runs the list
// forward. Quiesce and stop run it back, so both reverse orderings are that one
// list read backwards rather than structures of their own.
//
// A member's level is its position in Levels, and is not restated anywhere a
// filtered or reordered slice could make it a lie.
type InjectorAnalysis struct {
	Injector *Injector
	Levels   []Level
}

// Member returns where a component landed, and false when it occupies no level.
func (ia *InjectorAnalysis) Member(c *Component) (Member, bool) {
	for _, level := range ia.Levels {
		for _, m := range level {
			if m.Component == c {
				return m, true
			}
		}
	}

	return Member{}, false
}

// Analysis is one package's lifecycle analysis, one entry per injector in the
// order that the injectors were parsed. Injectors are independent graphs and
// are never merged.
type Analysis struct {
	Injectors []*InjectorAnalysis
}

// For returns the named injector's analysis, or nil when the package has no such
// injector.
func (a *Analysis) For(injector string) *InjectorAnalysis {
	for _, ia := range a.Injectors {
		if ia.Injector.Name == injector {
			return ia
		}
	}

	return nil
}

// computeLevels places every component that occupies a level into one. A
// component's level is one past the deepest level held by any occupant that
// it depends on, and level 0 when it depends on none, so a dependent is
// always in a strictly later level than its dependencies and components with
// no dependency between them share a level. The levels that it returns are
// contiguous.
//
// A component that occupies no level still transmits ordering: an occupant that
// reaches another only through such components is still placed after it.
//
// The result is deterministic. Components are walked in the statement order Wire
// emitted them, which is a valid topological order, and each level's members stay
// in that order.
func computeLevels(inj *Injector, caps map[*Component]Capabilities) *InjectorAnalysis {
	ia := &InjectorAnalysis{Injector: inj}

	// The number of levels that a component sits behind: one past its own level
	// when it occupies one, and the deepest such count among its dependencies
	// when it does not, which is how ordering crosses a non-occupying component.
	behind := make(map[*Component]int, len(inj.Components))

	for _, c := range inj.Components {
		after := 0
		for _, d := range c.Deps {
			if behind[d] > after {
				after = behind[d]
			}
		}

		if !occupiesLevel(c, caps[c]) {
			behind[c] = after
			continue
		}
		behind[c] = after + 1

		for len(ia.Levels) <= after {
			ia.Levels = append(ia.Levels, nil)
		}

		m := Member{Component: c, Capabilities: caps[c]}
		ia.Levels[after] = append(ia.Levels[after], m)
	}

	return ia
}

// occupiesLevel reports whether a component takes a place of its own in the
// level list. Every lifecycle-capable component does, and so does a
// dependency-only component with a provider that returned a cleanup. That
// cleanup runs at the dependency-only component's position. A component
// with neither trait occupies none.
func occupiesLevel(c *Component, caps Capabilities) bool {
	return caps != None || c.Cleanup != nil
}
