// Copyright the original author or authors.
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

package graph_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/internal/generator/graph"
)

// closerTypes declares the types that the closer specs build. Conn declares
// Close on the pointer. Flat declares Close on the value. Both declares Close
// and Stop. Plain declares neither.
const closerTypes = `package app

import "context"

type Conn struct{}

func (*Conn) Close() error { return nil }

type Flat struct{}

func (Flat) Close() error { return nil }

type Both struct{}

func (*Both) Close() error         { return nil }
func (*Both) Stop(context.Context) {}

type Plain struct{}

type App struct{}
`

// closerFile is the file that each spec writes its providers, its provider
// set, and its injector into.
const closerFile = "graph.go"

// closerHeader starts closerFile.
const closerHeader = "package app\n\nimport \"github.com/google/wire\"\n\nvar _ = wire.NewSet\n\n"

var _ = Describe("closer directives", func() {
	var dir string

	BeforeEach(func() {
		// The package must sit inside this module, so that the load resolves
		// Google Wire from this module's go.mod.
		Expect(os.MkdirAll("testdata", 0o750)).To(Succeed())

		var err error
		dir, err = os.MkdirTemp("testdata", "closer-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = os.RemoveAll(dir)
		})

		Expect(os.WriteFile(filepath.Join(dir, "types.go"), []byte(closerTypes), 0o600)).To(Succeed())
	})

	// detect writes body as the package's graph file. body declares the
	// providers and the function Init, which stands in for the injector that
	// Google Wire writes.
	detect := func(body string) (graph.Detected, error) {
		src := closerHeader + body
		Expect(os.WriteFile(filepath.Join(dir, closerFile), []byte(src), 0o600)).To(Succeed())

		injectors, err := graph.Parse([]byte(src), []string{"Init"})
		Expect(err).NotTo(HaveOccurred())

		return graph.Detect(dir, nil, injectors)
	}

	// closerOf reports whether the named component is a closer.
	closerOf := func(detected graph.Detected, name string) bool {
		for _, c := range detected.Injectors[0].Components {
			if c.Name == name {
				return c.Closer
			}
		}

		Fail("no component named " + name)

		return false
	}

	// provider returns a provider of typ under the doc comment doc, with an
	// injector that calls it.
	provider := func(doc, typ, literal string) string {
		return doc + "func NewIt() " + typ + " { return " + literal + " }\n\n" +
			"func Init() " + typ + " {\n\tit := NewIt()\n\treturn it\n}\n"
	}

	Describe("a directive on a provider's declaration", func() {
		It("marks the component that the provider builds", func() {
			detected, err := detect(provider("// NewIt provides it.\n//\n//yama:closer\n", "*Conn", "&Conn{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "it")).To(BeTrue())
			Expect(detected.Warnings).To(BeEmpty())
		})

		It("marks a value with Close on the value type", func() {
			detected, err := detect(provider("//yama:closer\n", "Flat", "Flat{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "it")).To(BeTrue())
		})

		It("marks a pointer with Close on the value type", func() {
			detected, err := detect(provider("//yama:closer\n", "*Flat", "&Flat{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "it")).To(BeTrue())
		})

		It("marks a provider that another package declares", func() {
			body := "func Init() (*os.File, error) {\n\tfile, err := os.Open(\"name\")\n" +
				"\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn file, nil\n}\n"
			src := strings.Replace(closerHeader, "import \"github.com/google/wire\"",
				"import (\n\t\"os\"\n\n\t\"github.com/google/wire\"\n)", 1) + body
			Expect(os.WriteFile(filepath.Join(dir, closerFile), []byte(src), 0o600)).To(Succeed())

			injectors, err := graph.Parse([]byte(src), []string{"Init"})
			Expect(err).NotTo(HaveOccurred())

			detected, err := graph.Detect(dir, nil, injectors)
			Expect(err).NotTo(HaveOccurred())

			// os.Open carries no directive, so Yama read its declaration and
			// found an unmarked closer.
			Expect(closerOf(detected, "file")).To(BeFalse())
			Expect(detected.Warnings).To(ConsistOf(ContainSubstring("provider os.Open builds an io.Closer")))
		})

		It("reads a directive on a provider in another package of the module", func() {
			sub := filepath.Join(dir, "sub")
			Expect(os.Mkdir(sub, 0o750)).To(Succeed())

			subSrc := "package sub\n\ntype Conn struct{}\n\nfunc (*Conn) Close() error { return nil }\n\n" +
				"// NewConn provides the connection.\n//\n//yama:closer\nfunc NewConn() *Conn { return &Conn{} }\n"
			Expect(os.WriteFile(filepath.Join(sub, "sub.go"), []byte(subSrc), 0o600)).To(Succeed())

			subPath := "l7e.io/yama/internal/generator/graph/testdata/" + filepath.Base(dir) + "/sub"
			src := "package app\n\nimport \"" + subPath + "\"\n\n" +
				"func Init() *sub.Conn {\n\tconn := sub.NewConn()\n\treturn conn\n}\n"
			Expect(os.WriteFile(filepath.Join(dir, closerFile), []byte(src), 0o600)).To(Succeed())

			injectors, err := graph.Parse([]byte(src), []string{"Init"})
			Expect(err).NotTo(HaveOccurred())

			detected, err := graph.Detect(dir, nil, injectors)
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "conn")).To(BeTrue())
			Expect(detected.Warnings).To(BeEmpty())
		})

		DescribeTable("reports a directive that the provider cannot carry",
			func(doc, typ, literal, want string) {
				_, err := detect(provider(doc, typ, literal))

				Expect(err).To(MatchError(ContainSubstring(want)))
			},
			Entry("a name that it does not know", "//yama:close\n", "*Conn", "&Conn{}", "unknown directive //yama:close"),
			Entry("both directives", "//yama:closer\n//yama:noclose\n", "*Conn", "&Conn{}", "carries both"),
			Entry("closer on a type with no Close", "//yama:closer\n", "*Plain", "&Plain{}", "builds no io.Closer in Init"),
			Entry("noclose on a type with no Close", "//yama:noclose\n", "*Plain", "&Plain{}", "builds no io.Closer in Init"),
			Entry("closer on a value with Close on the pointer", "//yama:closer\n", "Conn", "Conn{}", "builds no io.Closer in Init"),
			Entry("closer on a type that declares Stop", "//yama:closer\n", "*Both", "&Both{}", "already declares Stop"),
		)

		It("reports a closer directive on a provider that returns a cleanup", func() {
			body := "//yama:closer\nfunc NewIt() (*Conn, func()) { return &Conn{}, func() {} }\n\n" +
				"func Init() (*Conn, func()) {\n\tit, cleanup := NewIt()\n\treturn it, func() {\n\t\tcleanup()\n\t}\n}\n"

			_, err := detect(body)

			Expect(err).To(MatchError(ContainSubstring("also returns a cleanup")))
		})

		It("names the file and the line of the directive", func() {
			_, err := detect(provider("//yama:close\n", "*Conn", "&Conn{}"))

			Expect(err).To(MatchError(ContainSubstring(closerFile + ":7:")))
		})
	})

	Describe("a directive on a wire.Struct entry", func() {
		// structGraph returns a graph that builds typ from a wire.Struct entry
		// under the comment lines above.
		structGraph := func(above, typ, literal string) string {
			return "var Set = wire.NewSet(\n" + above + "\twire.Struct(new(Conn), \"*\"))\n\n" +
				"func Init() " + typ + " {\n\tconn := " + literal + "\n\treturn conn\n}\n"
		}

		It("marks the pointer that the graph builds", func() {
			detected, err := detect(structGraph("\t//yama:closer\n", "*Conn", "&Conn{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "conn")).To(BeTrue())
		})

		It("reports a graph that builds only the value when Close is on the pointer", func() {
			_, err := detect(structGraph("\t//yama:closer\n", "Conn", "Conn{}"))

			Expect(err).To(MatchError(ContainSubstring("wire.Struct entry for Conn, which builds no io.Closer")))
		})

		It("marks only the forms that implement io.Closer when the graph builds both", func() {
			body := "var Set = wire.NewSet(\n\t//yama:closer\n\twire.Struct(new(Conn), \"*\"))\n\n" +
				"func NewApp(v Conn, p *Conn) *App { return &App{} }\n\n" +
				"func Init() *App {\n\tconn := Conn{}\n\tconn2 := &Conn{}\n\tapp := NewApp(conn, conn2)\n\treturn app\n}\n"

			detected, err := detect(body)
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "conn")).To(BeFalse())
			Expect(closerOf(detected, "conn2")).To(BeTrue())
			Expect(detected.Warnings).To(BeEmpty())
		})

		It("reports two entries for one type that do not carry the same directive", func() {
			body := "var Other = wire.NewSet(wire.Struct(new(Conn), \"*\"))\n\n" +
				structGraph("\t//yama:closer\n", "*Conn", "&Conn{}")

			_, err := detect(body)

			Expect(err).To(MatchError(ContainSubstring("carries //yama:closer, and the entry at")))
		})
	})

	Describe("placement in the target package", func() {
		conn := "func NewConn() *Conn { return &Conn{} }\n\n" +
			"func Init() *Conn {\n\tconn := NewConn()\n\treturn conn\n}\n"

		DescribeTable("reports a directive that marks nothing it can mark",
			func(body, want string) {
				_, err := detect(body + conn)

				Expect(err).To(MatchError(ContainSubstring(want)))
			},
			Entry("on a provider function's entry in a set",
				"var Set = wire.NewSet(\n\t//yama:closer\n\tNewConn)\n\n",
				"goes in the doc comment of the declaration of NewConn"),
			Entry("on an entry that is not a wire.Struct entry",
				"var Set = wire.NewSet(NewConn,\n\t//yama:closer\n\twire.Bind(new(error), new(*Conn)))\n\n",
				"is not a wire.Struct entry"),
			Entry("on the same line as the entry",
				"var Set = wire.NewSet(\n\twire.Struct(new(Conn), \"*\"), //yama:closer\n)\n\n",
				"must occupy its own line"),
			Entry("at the end of the argument list",
				"var Set = wire.NewSet(wire.Struct(new(Conn), \"*\"),\n\t//yama:closer\n)\n\n",
				"no entry follows"),
			Entry("on a variable declaration",
				"//yama:closer\nvar Set = wire.NewSet(NewConn)\n\n",
				"marks no function declaration and no wire.Struct entry"),
			Entry("on a method",
				"//yama:closer\nfunc (*App) Build() *Conn { return nil }\n\n",
				"a provider is never a method"),
		)
	})

	Describe("the warning for an unmarked closer", func() {
		It("names the provider that builds a closer with no directive", func() {
			detected, err := detect(provider("", "*Conn", "&Conn{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "it")).To(BeFalse())
			Expect(detected.Warnings).To(HaveLen(1))
			Expect(detected.Warnings[0]).To(ContainSubstring(closerFile + ":"))
			Expect(detected.Warnings[0]).To(ContainSubstring("provider NewIt builds an io.Closer that nothing closes"))
			Expect(detected.Warnings[0]).To(ContainSubstring("//yama:closer or //yama:noclose"))
		})

		It("names the wire.Struct entry that builds a closer with no directive", func() {
			body := "var Set = wire.NewSet(wire.Struct(new(Conn), \"*\"))\n\n" +
				"func Init() *Conn {\n\tconn := &Conn{}\n\treturn conn\n}\n"

			detected, err := detect(body)
			Expect(err).NotTo(HaveOccurred())

			Expect(detected.Warnings).To(ConsistOf(ContainSubstring("wire.Struct entry for Conn builds an io.Closer")))
		})

		It("stops for a provider that carries the noclose directive", func() {
			detected, err := detect(provider("//yama:noclose\n", "*Conn", "&Conn{}"))
			Expect(err).NotTo(HaveOccurred())

			Expect(closerOf(detected, "it")).To(BeFalse())
			Expect(detected.Warnings).To(BeEmpty())
		})

		DescribeTable("prints nothing",
			func(body string) {
				detected, err := detect(body)
				Expect(err).NotTo(HaveOccurred())

				Expect(detected.Warnings).To(BeEmpty())
			},
			Entry("for a type with no Close", provider("", "*Plain", "&Plain{}")),
			Entry("for a value with Close on the pointer", provider("", "Conn", "Conn{}")),
			Entry("for a type that declares Stop", provider("", "*Both", "&Both{}")),
			Entry("for a provider that returns a cleanup",
				"func NewIt() (*Conn, func()) { return &Conn{}, func() {} }\n\n"+
					"func Init() (*Conn, func()) {\n\tit, cleanup := NewIt()\n\treturn it, func() {\n\t\tcleanup()\n\t}\n}\n"),
			Entry("for a value that no provider function builds",
				"var _wireFileValue = &Conn{}\n\nfunc Init() *Conn {\n\tconn := _wireFileValue\n\treturn conn\n}\n"),
		)
	})
})
