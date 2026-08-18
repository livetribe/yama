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

package work

import (
	"regexp"
	"strconv"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

// These are the paths of Yama's own two packages. Every lifecycle file imports
// them.
const (
	yamaPath = "l7e.io/yama/v2"
	rtPath   = "l7e.io/yama/v2/rt"
)

// blankName binds nothing. A parameter that carries it states no name that the
// constructor body can reach.
const blankName = "_"

// naming holds the names that one lifecycle file uses for Yama's own two
// packages. It also holds the name that each constructor uses to forward its
// options.
//
// The lifecycle file shares a block with the package that contains it. Each
// constructor shares a scope with the values that its body builds. Yama's own
// imports cannot take a name that the block or a scope holds.
type naming struct {
	yama string
	rt   string

	// opts holds one name for each stub, in the same order as the package
	// declares the stubs. The entry is empty for a stub that declared no
	// options.
	opts []string
}

// nameFile decides what the lifecycle file calls Yama's own two packages, and
// what each constructor calls its options parameter.
//
// An options parameter takes a different name if any other name that its
// constructor reaches already holds it. Those other names are the package
// block, a component, a cleanup, an import, and the parameters that the stub
// declares beside the options. Yama's own two imports take a different name
// for all of those names, and for the options parameters also.
func nameFile(imports []pkg.Import, injectors []graph.Injector, scope []string, stubs []pkg.Stub) naming {
	taken := make(map[string]bool, len(scope))
	for _, name := range scope {
		taken[name] = true
	}

	for _, inj := range injectors {
		for _, c := range inj.Components {
			taken[c.Name] = true

			if c.Cleanup != "" {
				taken[c.Cleanup] = true
			}
		}
	}

	// These are the names of the file's own import block. Yama's own two
	// imports are not in this set. nameFile decides the names that they
	// take.
	for _, imp := range imports {
		if imp.Path == yamaPath || imp.Path == rtPath {
			continue
		}

		taken[imp.Name] = true
	}

	takeParams(taken, stubs)

	n := naming{opts: nameOptions(taken, stubs)}

	for _, name := range n.opts {
		if name != "" {
			taken[name] = true
		}
	}

	n.yama = freeName("yama", taken)
	taken[n.yama] = true

	n.rt = freeName("rt", taken)

	return n
}

// takeParams marks every parameter that a stub declares beside its options. A
// constructor keeps each of those names. Each of those names shares a scope
// with the constructor body.
func takeParams(taken map[string]bool, stubs []pkg.Stub) {
	for i := range stubs {
		for _, p := range stubs[i].GraphParams() {
			if p.Name == "" || p.Name == blankName {
				continue
			}

			taken[p.Name] = true
		}
	}
}

// nameOptions returns the name that each stub's constructor uses to forward
// its options. It returns an empty entry for a stub that declared no options.
//
// One constructor's options parameter shares a scope with no other
// constructor's options parameter. Each name must therefore differ only from
// the names in taken.
func nameOptions(taken map[string]bool, stubs []pkg.Stub) []string {
	opts := make([]string, len(stubs))

	for i := range stubs {
		name := stubs[i].OptionsName()
		if name == "" {
			continue
		}

		opts[i] = freeName(name, taken)
	}

	return opts
}

// freeName returns want, or want with the lowest number that no name in taken
// already holds.
func freeName(want string, taken map[string]bool) string {
	if !taken[want] {
		return want
	}

	for i := 2; ; i++ {
		candidate := want + strconv.Itoa(i)

		if !taken[candidate] {
			return candidate
		}
	}
}

// requalify returns the type under a new package name. In that type, to
// replaces every reference to from.
func requalify(typ, from, to string) string {
	if from == "" || from == to {
		return typ
	}

	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(from) + `\.`)

	return pattern.ReplaceAllString(typ, to+".")
}
