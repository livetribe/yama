package wire_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/source"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// pkg is one target package that declares one stub taking options.
func pkg() *source.PackageInfo {
	return &source.PackageInfo{
		Name: "app",
		Imports: []source.Import{
			{Path: "context"},
			{Path: "github.com/google/wire"},
			{Name: "applib", Path: "example.com/app/lib"},
		},
		Stubs: []source.StubInfo{
			{
				Name: "NewAppLifecycle",
				Params: []source.Field{
					{Name: "ctx", Type: "context.Context"},
					{Name: "opts", Type: "...yama.Option"},
				},
				Results: []source.Field{
					{Type: "*applib.Lifecycle"},
					{Type: "error"},
				},
				Providers: []string{"applib.NewDB", "applib.ProviderSet"},
				HasOpts:   true,
			},
		},
	}
}

// positioned is the same package, and its stub states the file and the position
// that a person wrote it at.
func positioned() *source.PackageInfo {
	p := pkg()
	p.Stubs[0].File = "lifecycle.go"
	p.Stubs[0].Line = 27
	p.Stubs[0].Column = 1

	return p
}

var _ = Describe("wire", func() {
	Describe("DerivedName", func() {
		It("starts the stub's name with a prefix of its own", func() {
			Expect(wire.DerivedName("NewAppLifecycle")).To(Equal("yama_NewAppLifecycle"))
		})
	})

	Describe("OutputName", func() {
		It("returns the plain name when the run set no prefix", func() {
			Expect(wire.OutputName("")).To(Equal("wire_gen.go"))
		})

		It("starts the name with the run's prefix", func() {
			Expect(wire.OutputName("foo_")).To(Equal("foo_wire_gen.go"))
		})
	})

	Describe("Derive", func() {
		It("assembles a file that go/format accepts", func() {
			Expect(string(wire.Derive(pkg()))).To(Equal(`//go:build wireinject

package app

import (
	context "context"
	applib "example.com/app/lib"
	wire "github.com/google/wire"
)

func yama_NewAppLifecycle(ctx context.Context) (*applib.Lifecycle, func(), error) {
	panic(wire.Build(applib.NewDB, applib.ProviderSet))
}
`))
		})

		It("lets go/format sort the import block by path", func() {
			derived := string(wire.Derive(pkg()))

			Expect(derived).To(ContainSubstring("\tcontext \"context\"\n\tapplib \"example.com/app/lib\"\n\twire \"github.com/google/wire\"\n"))
		})

		It("guards the file with the tag that only Google Wire sets", func() {
			Expect(string(wire.Derive(pkg()))).To(HavePrefix("//go:build wireinject"))
		})

		Context("the derived injector", func() {
			It("drops the stub's trailing option parameter", func() {
				Expect(string(wire.Derive(pkg()))).To(ContainSubstring("func yama_NewAppLifecycle(ctx context.Context)"))
			})

			It("keeps the stub's provider list as it was written", func() {
				Expect(string(wire.Derive(pkg()))).To(ContainSubstring("wire.Build(applib.NewDB, applib.ProviderSet)"))
			})

			It("returns the stub's first result, a cleanup, and an error", func() {
				derived := string(wire.Derive(pkg()))

				Expect(derived).To(ContainSubstring("(*applib.Lifecycle, func(), error)"))
			})

			It("drops the stub's own lifecycle result, which Google Wire rejects", func() {
				p := pkg()
				p.Stubs[0].Results = []source.Field{
					{Type: "*applib.App"},
					{Type: "yama.Lifecycle"},
					{Type: "error"},
				}

				derived := string(wire.Derive(p))

				Expect(derived).To(ContainSubstring("(*applib.App, func(), error)"))
				Expect(derived).NotTo(ContainSubstring("yama.Lifecycle"))
			})
		})

		Context("a stub that takes no options", func() {
			It("gives the injector the same parameters as the stub", func() {
				p := pkg()
				p.Stubs[0].HasOpts = false
				p.Stubs[0].Params = []source.Field{{Name: "ctx", Type: "context.Context"}}

				Expect(string(wire.Derive(p))).To(ContainSubstring("func yama_NewAppLifecycle(ctx context.Context)"))
			})
		})

		Context("a package that declares several stubs", func() {
			It("writes an injector for each", func() {
				p := pkg()
				p.Stubs = append(p.Stubs, source.StubInfo{
					Name:      "NewOther",
					Results:   []source.Field{{Type: "error"}},
					Providers: []string{"applib.Other"},
				})

				derived := string(wire.Derive(p))

				Expect(derived).To(ContainSubstring("func yama_NewAppLifecycle("))
				Expect(derived).To(ContainSubstring("func yama_NewOther("))
			})

			It("writes no placeholder", func() {
				Expect(string(wire.Derive(pkg()))).NotTo(ContainSubstring("func NewAppLifecycle("))
			})
		})

		Context("an import that the path does not name", func() {
			// gopkg.in/yaml.v3 declares the package yaml. A file that refers to
			// yaml.Node must import the path under that name, and no rule over
			// the path alone produces it.
			It("writes the name that the package declares", func() {
				p := pkg()
				p.Imports = append(p.Imports, source.Import{Path: "gopkg.in/yaml.v3"})
				p.Stubs[0].Results = []source.Field{{Type: "*yaml.Node"}, {Type: "error"}}

				p.ImportNames = map[string]string{"gopkg.in/yaml.v3": "yaml"}

				Expect(string(wire.Derive(p))).To(ContainSubstring(`yaml "gopkg.in/yaml.v3"`))
			})

			It("drops the import when no name it holds matches the code", func() {
				p := pkg()
				p.Imports = append(p.Imports, source.Import{Path: "gopkg.in/yaml.v3"})

				Expect(string(wire.Derive(p))).NotTo(ContainSubstring("gopkg.in/yaml.v3"))
			})
		})

		// Google Wire loads the derived file through go/packages, which applies
		// a line directive. Google Wire then reports the file that a person
		// wrote, at the position that person wrote the stub at.
		It("puts each declaration at the position of the stub it comes from", func() {
			Expect(string(wire.Derive(positioned()))).To(ContainSubstring("\n//line lifecycle.go:27:1\n"))
		})

		Context("a package that imports nothing", func() {
			It("writes no import block", func() {
				p := pkg()
				p.Imports = nil
				p.Stubs[0].Params = nil
				p.Stubs[0].Results = []source.Field{{Type: "error"}}
				p.Stubs[0].HasOpts = false
				p.Stubs[0].Providers = []string{"One"}

				Expect(string(wire.Derive(p))).NotTo(ContainSubstring("import"))
			})
		})
	})

	Describe("DerivePlaceholder", func() {
		It("assembles a file that go/format accepts", func() {
			Expect(string(wire.DerivePlaceholder(pkg()))).To(Equal(`//go:build !yamainject

package app

import (
	context "context"
	applib "example.com/app/lib"
)

func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*applib.Lifecycle, error) {
	panic("yama: NewAppLifecycle is not generated")
}
`))
		})

		It("leaves out Google Wire, which a placeholder never calls", func() {
			Expect(string(wire.DerivePlaceholder(pkg()))).NotTo(ContainSubstring("github.com/google/wire"))
		})

		It("carries the build constraint that the lifecycle file carries", func() {
			Expect(string(wire.DerivePlaceholder(pkg()))).To(HavePrefix("//go:build !yamainject"))
		})

		It("declares the stub's own name and full parameter list", func() {
			placeholder := string(wire.DerivePlaceholder(pkg()))

			Expect(placeholder).To(ContainSubstring("func NewAppLifecycle(ctx context.Context, opts ...yama.Option)"))
		})

		It("panics with a message that names the stub", func() {
			Expect(string(wire.DerivePlaceholder(pkg()))).To(ContainSubstring(`panic("yama: NewAppLifecycle is not generated")`))
		})

		It("writes no derived injector", func() {
			Expect(string(wire.DerivePlaceholder(pkg()))).NotTo(ContainSubstring("yama_NewAppLifecycle"))
		})

		It("puts each declaration at the position of the stub it comes from", func() {
			Expect(string(wire.DerivePlaceholder(positioned()))).To(ContainSubstring("\n//line lifecycle.go:27:1\n"))
		})

		Context("a package that declares several stubs", func() {
			It("declares each stub's own name", func() {
				p := pkg()
				p.Stubs = append(p.Stubs, source.StubInfo{
					Name:      "NewOther",
					Results:   []source.Field{{Type: "error"}},
					Providers: []string{"applib.Other"},
				})

				placeholder := string(wire.DerivePlaceholder(p))

				Expect(placeholder).To(ContainSubstring("func NewAppLifecycle("))
				Expect(placeholder).To(ContainSubstring("func NewOther("))
			})
		})
	})

	Describe("Args", func() {
		It("returns no flag when a run set none", func() {
			Expect(wire.Args{}.Flags()).To(BeEmpty())
		})

		It("returns one flag for each value a run set", func() {
			args := wire.Args{HeaderFile: "header.txt", Tags: "special", Prefix: "foo_"}

			Expect(args.Flags()).To(Equal([]string{
				"-header_file=header.txt",
				"-tags=special",
				"-output_file_prefix=foo_",
			}))
		})

		It("leaves out a value that a run did not set", func() {
			Expect(wire.Args{Tags: "special"}.Flags()).To(Equal([]string{"-tags=special"}))
		})
	})

	Describe("Run", func() {
		It("starts no subprocess when a run resolved no pattern", func() {
			logged, err := wire.Run(context.Background(), GinkgoT().TempDir(), nil, wire.Args{})

			Expect(err).NotTo(HaveOccurred())
			Expect(logged).To(BeEmpty())
		})

		It("reports the directory it could not run in", func() {
			_, err := wire.Run(context.Background(), "/nonexistent", []string{"."}, wire.Args{})

			Expect(err).To(MatchError(ContainSubstring("/nonexistent")))
		})

		It("returns what Google Wire printed", func() {
			logged, err := wire.Run(context.Background(), GinkgoT().TempDir(), []string{"."}, wire.Args{})

			Expect(err).To(HaveOccurred())
			Expect(logged).NotTo(BeEmpty())
		})
	})
})
