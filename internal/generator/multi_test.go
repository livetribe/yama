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
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// injectorNames extracts and sorts the injector function names across a set of
// resolved packages, so a spec can assert on package identity regardless of the
// order Generate happened to visit directories in.
func injectorNames(pkgs []*LoadedPackage) []string {
	var names []string
	for _, pkg := range pkgs {
		for _, inj := range pkg.Injectors {
			names = append(names, inj.Name)
		}
	}
	sort.Strings(names)

	return names
}

var _ = Describe("ResolvePackages", func() {
	var ctx context.Context

	BeforeEach(func() {
		skipWithoutGo()
		ctx = context.Background()
	})

	It("expands a pattern to every package directory it matches", func() {
		dir := filepath.Join("testdata", "multipkg")

		dirs, err := ResolvePackages(ctx, dir, []string{"./..."}, Options{}.discoveryBuildFlags())
		Expect(err).NotTo(HaveOccurred())

		var bases []string
		for _, d := range dirs {
			bases = append(bases, filepath.Base(d))
		}
		Expect(bases).To(ConsistOf("a", "b", "noinjector", "bad"))
	})

	It("defaults to \".\" when no patterns are given", func() {
		dir := filepath.Join("testdata", "multipkg", "a")

		dirs, err := ResolvePackages(ctx, dir, nil, Options{}.discoveryBuildFlags())
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(HaveLen(1))
	})

	// Google Wire loads under the wireinject tag, because an injector only exists
	// under it. Resolution must match Wire's package set: a directory missed here
	// is one Wire writes to without a scope protecting it.
	It("resolves a package whose injector is only visible under wireinject", func() {
		dir := filepath.Join("testdata", "tagged")

		dirs, err := ResolvePackages(ctx, dir, []string{"."}, Options{Tags: "special"}.discoveryBuildFlags())
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(HaveLen(1))
	})

	// The caller's tags are appended to wireinject rather than replacing it, so a
	// tag-gated provider and the injector are visible at once.
	It("appends the caller's tags to wireinject rather than replacing it", func() {
		flags := Options{Tags: "special"}.discoveryBuildFlags()
		Expect(flags).To(Equal([]string{"-tags=wireinject special"}))

		Expect(Options{}.discoveryBuildFlags()).To(Equal([]string{"-tags=wireinject"}),
			"wireinject is set even with no caller tags")

		Expect(Options{}.parseBuildFlags()).To(BeEmpty(),
			"parsing the generated file must not set wireinject: it is //go:build !wireinject")
	})

	// A lifecycle stub only exists under yamainject, and the file Yama emits for
	// it is //go:build !yamainject, so one tag both reveals the stubs and hides
	// the constructors emitted for them.
	It("appends the caller's tags to yamainject to read stubs", func() {
		Expect(Options{}.stubBuildFlags()).To(Equal([]string{"-tags=yamainject"}))
		Expect(Options{Tags: "special"}.stubBuildFlags()).To(Equal([]string{"-tags=yamainject special"}))
	})
})

var _ = Describe("GenerateAll", func() {
	var (
		ctx context.Context
		dir string
	)

	BeforeEach(func() {
		skipWithoutGo()
		ctx = context.Background()
		dir = filepath.Join("testdata", "multipkg")
	})

	// bad fails Wire's own solver, so the sweep fails. Wire writes the packages
	// that succeed before reporting the one that did not, but those files are
	// transient here and the restore removes them: the run yields nothing rather
	// than a tree where some packages were regenerated and others were not.
	It("fails the whole run when Wire fails on any package", func() {
		pkgs, err := NewGenerator(Options{}).GenerateAll(ctx, dir, []string{"./..."})

		Expect(err).To(HaveOccurred(), "bad has no provider for *Base")
		Expect(err.Error()).To(ContainSubstring("no provider found"))
		Expect(pkgs).To(BeEmpty())
	})

	// a and b both have injectors and noinjector has none, so a clean sweep
	// yields the two that do and silently skips the one that does not.
	It("generates every package with an injector and skips the one without", func() {
		pkgs, err := NewGenerator(Options{}).GenerateAll(ctx, dir, []string{"./a", "./b", "./noinjector"})

		Expect(err).NotTo(HaveOccurred())
		Expect(injectorNames(pkgs)).To(Equal([]string{"InitRoot", "InitRoot"}))
	})

	It("leaves no trace in any of the swept directories", func() {
		_, err := NewGenerator(Options{}).GenerateAll(ctx, dir, []string{"./..."})
		Expect(err).To(HaveOccurred())

		for _, name := range []string{"a", "b", "noinjector", "bad"} {
			expectNoWireGenArtifacts(filepath.Join("testdata", "multipkg", name), Options{})
		}
	})

	// Wire reads -header_file once, relative to the directory it runs in. Running
	// Wire per-package instead pointed a relative path at each package directory,
	// so a header beside the caller could not be found.
	It("resolves a relative header file against the invoking directory", func() {
		header := filepath.Join(dir, "shared_header.txt")
		Expect(os.WriteFile(header, []byte("// shared header\n"), 0o600)).To(Succeed())
		DeferCleanup(func() { os.Remove(header) })

		opts := Options{HeaderFile: "shared_header.txt"}

		_, err := NewGenerator(opts).GenerateAll(ctx, dir, []string{"./a"})
		Expect(err).NotTo(HaveOccurred(),
			"the header sits beside the invoking directory, not inside package a")
	})

	// A partial open would leave the directories already scoped holding their
	// originals under backup names, with no caller left to put them back.
	It("rolls back the scopes it opened when a later one fails", func() {
		stale := filepath.Join(dir, "b", backupNameFor(Options{}.wireGenName()))
		Expect(os.WriteFile(stale, []byte("original\n"), 0o600)).To(Succeed())
		DeferCleanup(func() { os.Remove(stale) })

		_, err := NewGenerator(Options{}).GenerateAll(ctx, dir, []string{"./..."})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("interrupted run"))

		// Directories are scoped in sorted order, so "a" was opened before "b"
		// refused and must have been rolled back; the rest were never reached.
		for _, name := range []string{"a", "bad", "noinjector"} {
			expectNoWireGenArtifacts(filepath.Join(dir, name), Options{})
		}

		gen, _ := wireGenPathsFor(filepath.Join(dir, "b"), Options{})
		Expect(gen).NotTo(BeAnExistingFile(), "wire never ran, so nothing was generated")

		content, err := os.ReadFile(stale)
		Expect(err).NotTo(HaveOccurred(), "the stale backup itself is left untouched")
		Expect(string(content)).To(Equal("original\n"))
	})

	It("does not surface ErrNoInjectors in the joined error", func() {
		_, err := NewGenerator(Options{}).GenerateAll(ctx, dir, []string{"./..."})
		Expect(errors.Is(err, ErrNoInjectors)).To(BeFalse(),
			"a package with no injector is a silent skip, not a reported failure")
	})
})
