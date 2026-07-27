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

package yama_test

import (
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// updateGolden regenerates the committed API-surface snapshots when set:
//
//	go test -run TestAPISurface -update .
//
// The trailing "." matters: -update is this test binary's own flag, not one
// go test recognizes, so a package pattern given after it on the command line
// is swallowed as a test-binary argument instead of a package to build.
var updateGolden = flag.Bool("update", false, "update the API-surface golden files")

// modulePath is this module's import path, used to turn a loaded package's
// PkgPath into a path relative to the module root.
const modulePath = "l7e.io/yama/v2"

// goldenDir holds one snapshot per public package, named by its import path
// relative to the module root.
const goldenDir = "testdata/api_surface"

// TestAPISurface is the frozen public-API snapshot for every importable
// package in this module. It discovers packages by walking ./..., keeps only
// the ones that are actually part of the public API — skipping internal/
// packages (Go's own compiler already forbids importing those from outside
// this module) and main packages (never importable at all) — and renders
// each one's exported symbols via go/types, so a composed interface's
// promoted methods (Lifecycle's Start/Stop, embedded from Starter and
// Stopper) are captured. A package's rendered surface is compared against
// testdata/api_surface/<relative import path>.golden.
//
// Each golden is complete and must not be extended by hand. A diff means
// that package's permanent public API moved. If the change is genuinely
// intended, regenerate with -update and review the diff deliberately. A new
// public package picks up its own golden automatically the first time
// -update runs — nothing needs to be wired in by name.
func TestAPISurface(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, "./...")
	require.NoError(t, err, "loading ./...")
	require.Zero(t, packages.PrintErrors(pkgs), "type errors while loading ./...")

	for _, pkg := range publicPackages(pkgs) {
		t.Run(goldenName(pkg.PkgPath), func(t *testing.T) {
			assertGoldenSurface(t, pkg)
		})
	}
}

// publicPackages returns the packages that make up this module's public API:
// every loaded package except internal/ packages, which Go's own import rule
// already keeps out of reach from outside the module, and main packages,
// which are never importable by anything.
func publicPackages(pkgs []*packages.Package) []*packages.Package {
	var out []*packages.Package
	for _, pkg := range pkgs {
		if pkg.Name == "main" || isInternal(pkg.PkgPath) {
			continue
		}
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PkgPath < out[j].PkgPath })
	return out
}

// isInternal reports whether pkgPath contains an "internal" path segment,
// mirroring the rule the go command itself enforces on import.
func isInternal(pkgPath string) bool {
	for _, seg := range strings.Split(pkgPath, "/") {
		if seg == "internal" {
			return true
		}
	}
	return false
}

// goldenName derives a golden filename (without extension) from a package's
// import path relative to the module root, so the snapshot reads as which
// package it covers rather than an encoded full import path.
func goldenName(pkgPath string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(pkgPath, modulePath), "/")
	if rel == "" {
		return "root"
	}
	return strings.ReplaceAll(rel, "/", "_")
}

// assertGoldenSurface renders pkg's public surface and compares it against
// (or, with -update, writes it to) its golden file.
func assertGoldenSurface(t *testing.T, pkg *packages.Package) {
	t.Helper()

	got := renderPublicSurface(pkg)
	goldenPath := filepath.Join(goldenDir, goldenName(pkg.PkgPath)+".golden")

	if *updateGolden {
		require.NoError(t, os.MkdirAll(goldenDir, 0o750), "creating %s", goldenDir)
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o600), "writing golden")
		t.Logf("updated %s", goldenPath)
		return
	}

	wantBytes, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "reading golden (run `go test -run TestAPISurface -update .` to create it)")

	assert.Equal(t, string(wantBytes), got, "public API surface of %s changed.\n"+
		"This surface is frozen; a diff means the permanent public API moved.\n"+
		"If the change is intended, run `go test -run TestAPISurface -update .` and review the diff.", pkg.PkgPath)
}

// renderPublicSurface renders a canonical, deterministic listing of pkg's
// exported symbols from its already-loaded type information.
func renderPublicSurface(pkg *packages.Package) string {
	scope := pkg.Types.Scope()

	// Short-name qualifier: render foreign packages by name (context.Context,
	// os.Signal, bridge.Config) rather than full import path, for a stable,
	// readable snapshot. The package under render renders unqualified.
	qual := func(p *types.Package) string {
		if p == pkg.Types {
			return ""
		}
		return p.Name()
	}

	var names []string
	for _, name := range scope.Names() {
		if token.IsExported(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var b strings.Builder
	fmt.Fprintf(&b, "package %s // %s\n\n", pkg.Name, pkg.PkgPath)
	for _, name := range names {
		renderObject(&b, scope.Lookup(name), qual)
	}
	return b.String()
}

func renderObject(b *strings.Builder, obj types.Object, qual types.Qualifier) {
	switch o := obj.(type) {
	case *types.TypeName:
		renderTypeName(b, o, qual)
	case *types.Func:
		fmt.Fprintf(b, "func %s%s\n\n", o.Name(), signatureString(o.Type().(*types.Signature), qual))
	case *types.Var:
		fmt.Fprintf(b, "var %s %s\n\n", o.Name(), types.TypeString(o.Type(), qual))
	case *types.Const:
		fmt.Fprintf(b, "const %s %s = %s\n\n", o.Name(), types.TypeString(o.Type(), qual), o.Val().String())
	default:
		fmt.Fprintf(b, "%s\n\n", o.String())
	}
}

func renderTypeName(b *strings.Builder, tn *types.TypeName, qual types.Qualifier) {
	// An alias is rendered as "type Name = <target>" followed by an underlying-
	// kind note and its method set, so a method reachable through an alias cannot
	// change unnoticed. Nothing in the surface is an alias today; this branch
	// keeps the snapshot faithful if one is ever introduced.
	if tn.IsAlias() {
		target := types.TypeString(types.Unalias(tn.Type()), qual)
		fmt.Fprintf(b, "type %s = %s\n", tn.Name(), target)
		renderUnderlyingNote(b, tn.Type().Underlying(), qual)
		renderMethodSet(b, tn.Type(), qual)
		b.WriteString("\n")
		return
	}

	switch u := tn.Type().Underlying().(type) {
	case *types.Interface:
		fmt.Fprintf(b, "type %s interface {\n", tn.Name())
		renderInterface(b, u, qual)
		b.WriteString("}\n\n")
	case *types.Struct:
		fmt.Fprintf(b, "type %s struct {\n", tn.Name())
		renderStructFields(b, u, qual)
		b.WriteString("}\n")
		renderMethodSet(b, tn.Type(), qual)
		b.WriteString("\n")
	default:
		fmt.Fprintf(b, "type %s %s\n", tn.Name(), types.TypeString(u, qual))
		renderMethodSet(b, tn.Type(), qual)
		b.WriteString("\n")
	}
}

// renderUnderlyingNote emits a one-line note about an aliased type's underlying
// kind, including whether a struct exposes any exported fields.
func renderUnderlyingNote(b *strings.Builder, u types.Type, qual types.Qualifier) {
	switch st := u.(type) {
	case *types.Struct:
		exported := 0
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Exported() {
				exported++
			}
		}
		if exported == 0 {
			b.WriteString("\t// underlying: struct with no exported fields\n")
			return
		}
		b.WriteString("\t// underlying: struct\n")
		renderStructFields(b, st, qual)
	case *types.Interface:
		b.WriteString("\t// underlying: interface\n")
		renderInterface(b, st, qual)
	default:
		fmt.Fprintf(b, "\t// underlying: %s\n", types.TypeString(u, qual))
	}
}

func renderInterface(b *strings.Builder, iface *types.Interface, qual types.Qualifier) {
	var methods []string
	sealed := false
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		if !m.Exported() {
			sealed = true
			continue
		}
		methods = append(methods, fmt.Sprintf("\t%s%s", m.Name(), signatureString(m.Type().(*types.Signature), qual)))
	}
	sort.Strings(methods)
	for _, m := range methods {
		b.WriteString(m + "\n")
	}
	if sealed {
		b.WriteString("\t// sealed: has unexported method(s)\n")
	}
}

func renderStructFields(b *strings.Builder, st *types.Struct, qual types.Qualifier) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		fmt.Fprintf(b, "\t%s %s\n", f.Name(), types.TypeString(f.Type(), qual))
	}
}

// renderMethodSet lists the exported methods reachable on *T (which includes
// value-receiver and promoted methods), so a named type's full method set is
// captured here, not just the methods declared syntactically on it.
func renderMethodSet(b *strings.Builder, t types.Type, qual types.Qualifier) {
	ms := types.NewMethodSet(types.NewPointer(t))
	var lines []string
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i).Obj()
		if !m.Exported() {
			continue
		}
		lines = append(lines, fmt.Sprintf("\tmethod %s%s", m.Name(), signatureString(m.Type().(*types.Signature), qual)))
	}
	sort.Strings(lines)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
}

// signatureString renders a function/method signature as "(paramTypes) results"
// using type information only (no parameter names), for a stable snapshot.
func signatureString(sig *types.Signature, qual types.Qualifier) string {
	s := types.TypeString(sig, qual)
	return strings.TrimPrefix(s, "func") // -> "(params) results"
}
