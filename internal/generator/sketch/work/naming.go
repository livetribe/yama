package work

import (
	"regexp"
	"strconv"

	"l7e.io/yama/v2/internal/generator/sketch/emit"
	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/source"
)

// The paths of Yama's own two packages, which every lifecycle file imports.
const (
	yamaPath = "l7e.io/yama/v2"
	rtPath   = "l7e.io/yama/v2/rt"
)

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
// An options parameter that takes the name of an import the body needs gives
// way, because the body states that name and cannot state another. Yama's own
// two imports give way to everything else, because Yama writes every reference
// to them itself.
func nameFile(imports []emit.Import, injectors []graph.Injector, scope []string, stubs []source.StubInfo) naming {
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

	// imported names come from the file's own import block. Yama's own two are
	// not among them: this is what decides the names they take.
	imported := make(map[string]bool, len(imports))

	for _, imp := range imports {
		if imp.Path == yamaPath || imp.Path == rtPath {
			continue
		}

		imported[imp.Name] = true
		taken[imp.Name] = true
	}

	n := naming{opts: make([]string, len(stubs))}

	for i := range stubs {
		name := optsName(&stubs[i])
		if name == "" {
			continue
		}

		if imported[name] {
			name = freeName(name, taken)
		}

		taken[name] = true
		n.opts[i] = name
	}

	n.yama = freeName("yama", taken)
	taken[n.yama] = true

	n.rt = freeName("rt", taken)

	return n
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
