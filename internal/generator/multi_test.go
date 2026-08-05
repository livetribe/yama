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
	"path"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// emittedPkgs is the sorted last element of each emitted file's package path, so
// a spec can assert on package identity regardless of the order the sweep
// visited directories in.
func emittedPkgs(files []*LifecycleFile) []string {
	var names []string
	for _, f := range files {
		names = append(names, path.Base(f.PkgPath))
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

		dirs, err := ResolvePackages(ctx, dir, []string{"./..."}, Options{}.stubBuildFlags())
		Expect(err).NotTo(HaveOccurred())

		var bases []string
		for _, d := range dirs {
			bases = append(bases, filepath.Base(d))
		}
		Expect(bases).To(ConsistOf("a", "b", "noinjector", "bad"))
	})

	It("defaults to \".\" when no patterns are given", func() {
		dir := filepath.Join("testdata", "multipkg", "a")

		dirs, err := ResolvePackages(ctx, dir, nil, Options{}.stubBuildFlags())
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(HaveLen(1))
	})

	// A stub exists only under yamainject. Resolution must match the package set
	// a run generates for: a directory missed here is one Wire writes to without
	// a scope protecting it.
	It("resolves a package whose stub is only visible under yamainject", func() {
		dir := filepath.Join("testdata", "tagged")

		dirs, err := ResolvePackages(ctx, dir, []string{"."}, Options{Tags: "special"}.stubBuildFlags())
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(HaveLen(1))
	})

	It("sets no tag for the load that reads Wire's output", func() {
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

var _ = Describe("EmitAll over several packages", func() {
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
		files, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"./..."})

		Expect(err).To(HaveOccurred(), "bad has no provider for *Base")
		Expect(err.Error()).To(ContainSubstring("no provider found"))
		Expect(files).To(BeEmpty())
	})

	// a and b both declare a stub and noinjector declares none, so a clean sweep
	// yields the two that do and silently skips the one that does not.
	It("generates every package with a stub and skips the one without", func() {
		files, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"./a", "./b", "./noinjector"})

		Expect(err).NotTo(HaveOccurred())
		Expect(emittedPkgs(files)).To(Equal([]string{"a", "b"}),
			"the package with no stub contributes no file")
	})

	It("leaves no trace in any of the swept directories", func() {
		_, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"./..."})
		Expect(err).To(HaveOccurred())

		for _, name := range []string{"a", "b", "noinjector", "bad"} {
			expectNoTransientArtifacts(filepath.Join("testdata", "multipkg", name), Options{})
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

		_, err := NewGenerator(opts).EmitAll(ctx, dir, []string{"./a"})
		Expect(err).NotTo(HaveOccurred(),
			"the header sits beside the invoking directory, not inside package a")
	})

	// A partial open would leave the directories already scoped holding their
	// originals under backup names, with no caller left to put them back.
	It("rolls back the scopes it opened when a later one fails", func() {
		stale := filepath.Join(dir, "b", backupNameFor(Options{}.wireGenName()))
		Expect(os.WriteFile(stale, []byte("original\n"), 0o600)).To(Succeed())
		DeferCleanup(func() { os.Remove(stale) })

		_, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"./..."})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("interrupted run"))

		// Directories are scoped in sorted order, so "a" was opened before "b"
		// refused and must have been rolled back; the rest were never reached.
		for _, name := range []string{"a", "bad", "noinjector"} {
			expectNoTransientArtifacts(filepath.Join(dir, name), Options{})
		}

		gen, _ := wireGenPathsFor(filepath.Join(dir, "b"), Options{})
		Expect(gen).NotTo(BeAnExistingFile(), "wire never ran, so nothing was generated")

		content, err := os.ReadFile(stale)
		Expect(err).NotTo(HaveOccurred(), "the stale backup itself is left untouched")
		Expect(string(content)).To(Equal("original\n"))
	})

	It("does not surface ErrNoStubs in the joined error", func() {
		_, err := NewGenerator(Options{}).EmitAll(ctx, dir, []string{"./..."})
		Expect(errors.Is(err, ErrNoStubs)).To(BeFalse(),
			"a package with no stub is a silent skip, not a reported failure")
	})
})
