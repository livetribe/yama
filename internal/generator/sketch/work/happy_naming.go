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

// The paths of Yama's own two packages, which every lifecycle file imports.
const (
	yamaPath = "l7e.io/yama/v2"
	rtPath   = "l7e.io/yama/v2/rt"
)

// blankName binds nothing. A parameter that carries it states no name that the
// constructor body can reach.
const blankName = "_"

// naming holds the names that one lifecycle file refers to Yama's own two
// packages by, and the name each constructor forwards its options under.
//
// The lifecycle file shares a block with the package it sits in, and each
// constructor shares a scope with the values that its body builds. A name that
// any of those hold is a name that Yama's own imports cannot take.
type naming struct {
	yama string
	rt   string

	// opts holds one name for each stub, in the order the package declares
	// them. It is empty for a stub that declared no options.
	opts []string
}

// nameFile decides what the lifecycle file calls Yama's own two packages, and
// what each constructor calls its options parameter.
//
// An options parameter gives way to every other name that its constructor
// reaches: the package block, a component, a cleanup, an import, and the
// parameters that the stub declares beside it. Yama's own two imports give way
// to all of those and to the options parameters as well.
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

	// The names of the file's own import block. Yama's own two are not among
	// them: this is what decides the names they take.
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
// constructor keeps each of those names, and each one shares a scope with the
// constructor body.
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

// nameOptions returns the name that each stub's constructor forwards its
// options under, and an empty entry for a stub that declared none.
//
// One constructor's options parameter shares a scope with no other's, so each
// one gives way to taken alone.
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

// requalify returns the type with every reference to the stub's own name for
// Yama's public package replaced by the name that the lifecycle file uses.
func requalify(typ, from, to string) string {
	if from == "" || from == to {
		return typ
	}

	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(from) + `\.`)

	return pattern.ReplaceAllString(typ, to+".")
}
