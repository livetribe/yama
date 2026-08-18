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

package pkg_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/pkg"
)

// stubFile is one guarded file that declares one stub. Most specs write it and
// then check one property of what Load read back.
const stubFile = `//go:build yamainject

package app

import (
	"context"

	"example.com/app/lib"
	"l7e.io/yama/v2"
	"github.com/google/wire"
)

// NewAppLifecycle builds the lifecycle of app.
// It takes the request context.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*lib.App, yama.Lifecycle, error) {
	panic(wire.Build(lib.NewDB, lib.NewRepo, lib.ProviderSet))
}
`

var _ = Describe("source", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	// write puts content at name in dir.
	write := func(name, content string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
		Expect(err).NotTo(HaveOccurred())
	}

	// loaded reads dir and fails the spec when it cannot.
	loaded := func() *pkg.Info {
		pkg, err := pkg.Load(dir, nil)
		Expect(err).NotTo(HaveOccurred())

		return pkg
	}

	// only returns the one stub that dir declares.
	only := func() *pkg.Stub {
		pkg := loaded()
		Expect(pkg.Stubs()).To(HaveLen(1))

		return &pkg.Stubs()[0]
	}

	Describe("Load", func() {
		Context("when the directory declares one stub", func() {
			BeforeEach(func() {
				write("lifecycle.go", stubFile)
			})

			It("names the package", func() {
				Expect(loaded().Name()).To(Equal("app"))
			})

			It("reports the directory it read", func() {
				Expect(loaded().Dir()).To(Equal(dir))
			})

			It("names the stub", func() {
				Expect(only().Name).To(Equal("NewAppLifecycle"))
			})

			It("carries the doc comment without its markers", func() {
				Expect(only().Doc).To(Equal("NewAppLifecycle builds the lifecycle of app.\nIt takes the request context."))
			})

			It("reads the parameters, in order", func() {
				Expect(only().Params).To(Equal([]pkg.Field{
					{Name: "ctx", Type: "context.Context"},
					{Name: "opts", Type: "...yama.Option"},
				}))
			})

			It("reads the results, in order", func() {
				Expect(only().Results).To(Equal([]pkg.Field{
					{Type: "*lib.App"},
					{Type: "yama.Lifecycle"},
					{Type: "error"},
				}))
			})

			It("reads the providers as they were written", func() {
				Expect(only().Providers).To(Equal([]string{"lib.NewDB", "lib.NewRepo", "lib.ProviderSet"}))
			})

			It("reports that the stub takes options", func() {
				Expect(only().HasOpts).To(BeTrue())
			})
		})

		Context("when a file sets no build tag", func() {
			BeforeEach(func() {
				write("lifecycle.go", stubFile)
				write("ordinary.go", "package app\n\nfunc Other() {}\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(HaveLen(1))
			})
		})

		Context("when a file sets a different build tag", func() {
			BeforeEach(func() {
				write("other.go", "//go:build sometag\n\npackage app\n\nfunc Other() {}\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		Context("when a file carries Google Wire's own build constraint", func() {
			BeforeEach(func() {
				write("wire_gen.go", "//go:build !wireinject\n\npackage app\n\n"+
					"import \"example.com/other\"\n\nfunc InitApp() *other.App { return nil }\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})

			It("carries none of its imports onto the package", func() {
				Expect(loaded().Imports()).To(BeEmpty())
			})
		})

		Context("when a file negates a tag that no build sets", func() {
			BeforeEach(func() {
				write("other.go", "//go:build !sometag\n\npackage app\n\nfunc Other() {}\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		// A stub can be behind the run's own tag as well as Tag. Google Wire
		// receives that tag, and Load must read the same file that Google Wire
		// reads.
		Context("when a stub names one of the run's own tags beside the yamainject tag", func() {
			BeforeEach(func() {
				write("lifecycle.go", "//go:build yamainject && special\n\n"+
					"package app\n\nimport (\n\t\"github.com/google/wire\"\n\n\tyama \"l7e.io/yama/v2\"\n)\n\n"+
					"func NewThing() (*App, yama.Lifecycle, error) {\n\tpanic(wire.Build(One))\n}\n")
			})

			It("reads no stub when the run set no tag", func() {
				pkg, err := pkg.Load(dir, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(pkg.Stubs()).To(BeEmpty())
			})

			It("reads the stub when the run set that tag", func() {
				pkg, err := pkg.Load(dir, []string{"special"})

				Expect(err).NotTo(HaveOccurred())
				Expect(pkg.Stubs()).To(HaveLen(1))
			})

			It("reads no stub when the run set some other tag", func() {
				pkg, err := pkg.Load(dir, []string{"other"})

				Expect(err).NotTo(HaveOccurred())
				Expect(pkg.Stubs()).To(BeEmpty())
			})
		})

		// The run's own tags are set whether or not the yamainject tag is set. A
		// file that only names one of them is in an ordinary build, and it is
		// no stub.
		Context("when a file names one of the run's own tags alone", func() {
			BeforeEach(func() {
				write("other.go", "//go:build special\n\npackage app\n\nfunc Other() {}\n")
			})

			It("reads no stub out of it", func() {
				pkg, err := pkg.Load(dir, []string{"special"})

				Expect(err).NotTo(HaveOccurred())
				Expect(pkg.Stubs()).To(BeEmpty())
			})
		})

		Context("when a block comment sits above the build tag", func() {
			BeforeEach(func() {
				write("lifecycle.go", "/*\nCopyright the authors.\n*/\n\n"+stubFile)
			})

			It("reads the stub out of it", func() {
				Expect(loaded().Stubs()).To(HaveLen(1))
			})
		})

		Context("when a one-line block comment sits above the build tag", func() {
			BeforeEach(func() {
				write("lifecycle.go", "/* Copyright the authors. */\n\n"+stubFile)
			})

			It("reads the stub out of it", func() {
				Expect(loaded().Stubs()).To(HaveLen(1))
			})
		})

		Context("when a block comment sits below the package clause", func() {
			BeforeEach(func() {
				write("other.go", "package app\n\n/*\n//go:build yamainject\n*/\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		// A stub states its providers in a call to Google Wire's own Build. A
		// call to a Build function of any other package states none, and the
		// declaration that holds it belongs to the application.
		Context("when a guarded file calls Build on another package", func() {
			BeforeEach(func() {
				write("helper.go", "//go:build yamainject\n\npackage app\n\n"+
					"import \"example.com/query\"\n\n"+
					"func Report() (*Rows, error) {\n\tpanic(query.Build(\"select 1\"))\n}\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})

			It("reports no error for it", func() {
				_, err := pkg.Load(dir, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when a guarded file imports Google Wire under an alias", func() {
			BeforeEach(func() {
				write("lifecycle.go", "//go:build yamainject\n\npackage app\n\n"+
					"import (\n\tw \"github.com/google/wire\"\n\n\tyama \"l7e.io/yama/v2\"\n)\n\n"+
					"func NewApp() (*App, yama.Lifecycle, error) {\n\tpanic(w.Build(One))\n}\n")
			})

			It("reads the stub out of it", func() {
				Expect(loaded().Stubs()).To(HaveLen(1))
			})
		})

		Context("when a guarded file imports Google Wire not at all", func() {
			BeforeEach(func() {
				write("helper.go", "//go:build yamainject\n\npackage app\n\n"+
					"func Helper() (*App, error) {\n\tpanic(Build(One))\n}\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		Context("when the guarded file declares an ordinary function", func() {
			BeforeEach(func() {
				write("lifecycle.go", "//go:build yamainject\n\npackage app\n\nfunc Helper() int { return 1 }\n")
			})

			It("reads no stub out of it", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		Context("when a stub takes no options", func() {
			BeforeEach(func() {
				write("lifecycle.go", `//go:build yamainject

package app

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

func NewPlain() (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewDB))
}
`)
			})

			It("reports that the stub takes none", func() {
				Expect(only().HasOpts).To(BeFalse())
			})

			It("returns every parameter from GraphParams", func() {
				Expect(only().GraphParams()).To(BeEmpty())
			})
		})

		Context("when a stub declares several stubs in one file", func() {
			BeforeEach(func() {
				write("lifecycle.go", `//go:build yamainject

package app

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

func NewFirst() (*App, yama.Lifecycle, error) {
	panic(wire.Build(One))
}

func NewSecond() (*App, yama.Lifecycle, error) {
	panic(wire.Build(Two))
}
`)
			})

			It("reads them in the order the file declares them", func() {
				pkg := loaded()

				Expect(pkg.Stubs()).To(HaveLen(2))
				Expect(pkg.Stubs()[0].Name).To(Equal("NewFirst"))
				Expect(pkg.Stubs()[1].Name).To(Equal("NewSecond"))
			})
		})

		Context("when one declaration names several parameters of one type", func() {
			BeforeEach(func() {
				write("lifecycle.go", `//go:build yamainject

package app

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

func NewThing(a, b string) (*App, yama.Lifecycle, error) {
	panic(wire.Build(One))
}
`)
			})

			It("returns one field for each name", func() {
				Expect(only().Params).To(Equal([]pkg.Field{
					{Name: "a", Type: "string"},
					{Name: "b", Type: "string"},
				}))
			})
		})

		// A stub declares the signature that an emitted constructor needs. Load
		// reports a stub that does not declare it. Load does not let a later
		// phase write a file that no build accepts.
		Describe("the signature that a stub declares", func() {
			// guarded wraps a declaration in a guarded file that imports Yama.
			guarded := func(declaration string) string {
				return "//go:build yamainject\n\npackage app\n\n" +
					"import (\n\t\"github.com/google/wire\"\n\n\tyama \"l7e.io/yama/v2\"\n)\n\n" + declaration
			}

			It("takes the three results that a constructor returns", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, yama.Lifecycle, error) {\n\tpanic(wire.Build(One))\n}\n"))

				Expect(loaded().Stubs()).To(HaveLen(1))
			})

			It("reports a stub that declares too few results", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, error) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("declares 2 results")))
			})

			It("reports a stub that declares too many results", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, yama.Lifecycle, error, int) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("declares 4 results")))
			})

			It("reports a stub whose second result is not the lifecycle", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, func(), error) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("declares func() as its second result")))
			})

			It("reports a stub whose third result is not an error", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, yama.Lifecycle, int) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("declares int as its third result")))
			})

			It("reports a final option parameter that is not variadic", func() {
				write("lifecycle.go", guarded("func NewApp(opts yama.Option) (*App, yama.Lifecycle, error) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("declares yama.Option as its final parameter")))
			})

			It("takes a final variadic parameter that carries no option", func() {
				write("lifecycle.go", guarded("func NewApp(names ...string) (*App, yama.Lifecycle, error) {\n\tpanic(wire.Build(One))\n}\n"))

				Expect(loaded().Stubs()).To(HaveLen(1))
			})

			It("names the stub and the position that declares it", func() {
				write("lifecycle.go", guarded("func NewApp() (*App, error) {\n\tpanic(wire.Build(One))\n}\n"))

				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("lifecycle.go:")))
				Expect(err).To(MatchError(ContainSubstring("stub NewApp ")))
			})

			It("reads the alias that the file gave Yama's own package", func() {
				declaration := "//go:build yamainject\n\npackage app\n\n" +
					"import (\n\t\"github.com/google/wire\"\n\n\ty \"l7e.io/yama/v2\"\n)\n\n" +
					"func NewApp() (*App, y.Lifecycle, error) {\n\tpanic(wire.Build(One))\n}\n"

				write("lifecycle.go", declaration)

				Expect(loaded().Stubs()).To(HaveLen(1))
			})

			It("checks the signature of a stub alone", func() {
				write("lifecycle.go", guarded("func Helper() (int, int) { return 1, 2 }\n"))

				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		Context("when the directory declares no stub", func() {
			It("returns a package that holds none", func() {
				Expect(loaded().Stubs()).To(BeEmpty())
			})
		})

		Context("when a guarded file does not parse", func() {
			BeforeEach(func() {
				write("broken.go", "//go:build yamainject\n\npackage app\n\nfunc Broken( {\n")
			})

			It("names the file it could not read", func() {
				_, err := pkg.Load(dir, nil)

				Expect(err).To(MatchError(ContainSubstring("broken.go")))
			})
		})

		Context("when the directory is absent", func() {
			It("reports the directory it could not read", func() {
				_, err := pkg.Load(filepath.Join(dir, "absent"), nil)

				Expect(err).To(MatchError(ContainSubstring("absent")))
			})
		})

		Context("GraphParams", func() {
			BeforeEach(func() {
				write("lifecycle.go", stubFile)
			})

			It("drops the trailing option parameter", func() {
				Expect(only().GraphParams()).To(Equal([]pkg.Field{
					{Name: "ctx", Type: "context.Context"},
				}))
			})
		})
	})
})
