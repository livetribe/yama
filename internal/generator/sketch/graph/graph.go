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

// Package graph turns one injector's construction sequence into the lifecycle
// levels that a run emits. Every value it takes and every value it returns is a
// string or a plain struct of strings, so nothing from the Go toolchain crosses
// into the package that renders the file.
package graph

import "strings"

// A Capability is one lifecycle method that a component declares.
type Capability uint8

const (
	Start Capability = 1 << iota
	Quiesce
	Stop
)

// None names a component that declares no lifecycle method.
const None Capability = 0

// Has reports whether c declares every capability in other.
func (c Capability) Has(other Capability) bool {
	return c&other == other
}

// String names the capabilities in a fixed order, so the same set always prints
// the same way.
func (c Capability) String() string {
	if c == None {
		return "none"
	}

	var named []string

	for _, each := range []struct {
		bit  Capability
		name string
	}{{Start, "start"}, {Quiesce, "quiesce"}, {Stop, "stop"}} {
		if c.Has(each.bit) {
			named = append(named, each.name)
		}
	}

	return strings.Join(named, "|")
}

// A Component is one value that an injector builds. Deps names the components
// it consumes, and every name is the name of another component in the same
// injector. Cleanup is the name of the cleanup function that the provider
// returned, and it is empty when the provider returned none.
type Component struct {
	Name         string
	Deps         []string
	Capabilities Capability
	Cleanup      string
}

// A Member is one component's place in a level.
type Member struct {
	Name         string
	Capabilities Capability
	Cleanup      string
}

// Levels assigns each component that occupies a level to one level, and it
// returns the levels in the order a run starts them.
//
// A component sits one level behind the deepest level of anything it consumes.
// A component that occupies no level still carries ordering across itself: a
// component that reaches another only through such a component still lands
// after it.
//
// The result is deterministic. Levels walks components in the order it received
// them, which Google Wire already emitted in a valid topological order, and
// each level keeps that order.
func Levels(components []Component) [][]Member {
	var levels [][]Member

	// How many levels a component sits behind: one past its own level when it
	// occupies one, and the deepest count among its dependencies when it does
	// not.
	behind := make(map[string]int, len(components))

	for _, c := range components {
		after := deepest(c.Deps, behind)

		if !occupiesLevel(c) {
			behind[c.Name] = after

			continue
		}

		behind[c.Name] = after + 1

		for len(levels) <= after {
			levels = append(levels, nil)
		}

		member := Member{Name: c.Name, Capabilities: c.Capabilities, Cleanup: c.Cleanup}
		levels[after] = append(levels[after], member)
	}

	return levels
}

// deepest returns the greatest count that behind holds for any name in deps.
func deepest(deps []string, behind map[string]int) int {
	after := 0

	for _, d := range deps {
		if behind[d] > after {
			after = behind[d]
		}
	}

	return after
}

// occupiesLevel reports whether c takes a place of its own in the level list.
// Every component that declares a lifecycle method does. So does a component
// whose provider returned a cleanup, because that cleanup runs at the
// component's own position.
func occupiesLevel(c Component) bool {
	return c.Capabilities != None || c.Cleanup != ""
}
