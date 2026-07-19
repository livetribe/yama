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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// updateGolden regenerates the committed API-surface snapshot when set:
//
//	go test -run TestAPISurface -update ./...
var updateGolden = flag.Bool("update", false, "update the API-surface golden file")

const (
	pkgPath        = "l7e.io/yama/v2"
	goldenFilePath = "testdata/api_surface.golden"
)

// TestAPISurface is the frozen public-API snapshot for package yama. It
// enumerates every exported symbol of the package — via go/types, so a composed
// interface's promoted methods (Lifecycle's Start/Stop, embedded from Starter and
// Stopper) are captured — and fails if the rendered surface differs from
// testdata/api_surface.golden.
//
// This golden is complete and must not be extended. A diff means the
// permanent public API moved. If that change is genuinely intended,
// regenerate with -update and review the diff deliberately.
func TestAPISurface(t *testing.T) {
	got := renderPublicSurface(t)

	if *updateGolden {
		if err := os.WriteFile(goldenFilePath, []byte(got), 0o600); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("updated %s", goldenFilePath)
		return
	}

	wantBytes, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "reading golden (run `go test -run TestAPISurface -update` to create it)")

	assert.Equal(t, string(wantBytes), got, "public API surface changed.\n"+
		"This surface is frozen; a diff means the permanent public API moved.\n"+
		"If the change is intended, run `go test -run TestAPISurface -update` and review the diff.")
}

// renderPublicSurface loads package yama with full type information and renders a
// canonical, deterministic listing of its exported symbols.
func renderPublicSurface(t *testing.T) string {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	require.NoError(t, err, "loading %s", pkgPath)
	require.Zero(t, packages.PrintErrors(pkgs), "type errors while loading %s", pkgPath)
	require.Len(t, pkgs, 1)

	pkg := pkgs[0]
	scope := pkg.Types.Scope()

	// Short-name qualifier: render foreign packages by name (context.Context,
	// os.Signal, bridge.Config) rather than full import path, for a stable,
	// readable snapshot. The current package renders unqualified.
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
	fmt.Fprintf(&b, "package %s // %s\n\n", pkg.Name, pkgPath)
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
