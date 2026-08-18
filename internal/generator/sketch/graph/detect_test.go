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

package graph_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
)

// app is a package that declares one type for each capability shape that
// matters, and one injector that binds each of them.
const app = `package app

import "context"

type Full struct{}

func (Full) Start(ctx context.Context) error { return nil }
func (Full) Quiesce(ctx context.Context)     {}
func (Full) Stop(ctx context.Context)        {}

type Starter struct{}

func (Starter) Start(ctx context.Context) error { return nil }

type Pointer struct{}

func (p *Pointer) Stop(ctx context.Context) {}

type Embeds struct{ Starter }

type Plain struct{}

type WrongResult struct{}

func (WrongResult) Start(ctx context.Context) {}

type WrongParam struct{}

func (WrongParam) Stop() {}

type Named struct{}

func (Named) Quiesce(ctx context.Context, extra int) {}

func NewFull() Full              { return Full{} }
func NewStarter() Starter        { return Starter{} }
func NewPointer() *Pointer       { return &Pointer{} }
func NewValue() Pointer          { return Pointer{} }
func NewEmbeds() Embeds          { return Embeds{} }
func NewPlain() Plain            { return Plain{} }
func NewWrongResult() WrongResult { return WrongResult{} }
func NewWrongParam() WrongParam  { return WrongParam{} }
func NewNamed() Named            { return Named{} }

type App struct{}

func NewApp(
	full Full,
	starter Starter,
	pointer *Pointer,
	value Pointer,
	embeds Embeds,
	plain Plain,
	wrongResult WrongResult,
	wrongParam WrongParam,
	named Named,
) *App {
	return &App{}
}

func Init() *App {
	full := NewFull()
	starter := NewStarter()
	pointer := NewPointer()
	value := NewValue()
	embeds := NewEmbeds()
	plain := NewPlain()
	wrongResult := NewWrongResult()
	wrongParam := NewWrongParam()
	named := NewNamed()
	app := NewApp(full, starter, pointer, value, embeds, plain, wrongResult, wrongParam, named)
	return app
}
`

// unresolved is an injector that names a value that the package never declares.
// The load therefore resolves the type of no component in it.
const unresolved = `package app

func Ghost() *App {
	thing := NewMissing()
	app := NewApp2(thing)
	return app
}
`

var _ = Describe("Detect", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()

		write := func(name, content string) {
			Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)).To(Succeed())
		}

		write("go.mod", "module app\n\ngo 1.25\n")
		write("app.go", app)
	})

	// capsOf parses the injector, detects its capabilities, and returns them by
	// component name.
	capsOf := func() map[string]graph.Capability {
		injectors, err := graph.Parse([]byte(app), []string{"Init"})
		Expect(err).NotTo(HaveOccurred())

		filled, _, err := graph.Detect(dir, nil, injectors)
		Expect(err).NotTo(HaveOccurred())
		Expect(filled).To(HaveLen(1))

		out := make(map[string]graph.Capability)
		for _, c := range filled[0].Components {
			out[c.Name] = c.Capabilities
		}

		return out
	}

	// A component with a type that the load cannot resolve declares no
	// capability that Detect can read. Such a component occupies no lifecycle
	// level. A run that identified it as a plain value would leave it out of
	// the lifecycle.
	It("reports a component with a type that it cannot resolve", func() {
		injectors, err := graph.Parse([]byte(unresolved), []string{"Ghost"})
		Expect(err).NotTo(HaveOccurred())

		_, _, err = graph.Detect(dir, nil, injectors)

		Expect(err).To(MatchError(ContainSubstring("cannot resolve the type of")))
	})

	It("reads every capability a type declares", func() {
		Expect(capsOf()["full"]).To(Equal(graph.Start | graph.Quiesce | graph.Stop))
	})

	It("reads one capability a type declares", func() {
		Expect(capsOf()["starter"]).To(Equal(graph.Start))
	})

	It("reads none for a type that declares none", func() {
		Expect(capsOf()["plain"]).To(Equal(graph.None))
	})

	Context("a method with a pointer receiver", func() {
		It("belongs to the pointer type", func() {
			Expect(capsOf()["pointer"]).To(Equal(graph.Stop))
		})

		It("does not belong to the value type", func() {
			Expect(capsOf()["value"]).To(Equal(graph.None))
		})
	})

	Context("an embedded type", func() {
		It("promotes its capability to the type that embeds it", func() {
			Expect(capsOf()["embeds"]).To(Equal(graph.Start))
		})
	})

	Context("a method that only shares a capability's name", func() {
		It("is no capability when Start returns nothing", func() {
			Expect(capsOf()["wrongResult"]).To(Equal(graph.None))
		})

		It("is no capability when Stop takes no context", func() {
			Expect(capsOf()["wrongParam"]).To(Equal(graph.None))
		})

		It("is no capability when Quiesce takes more than a context", func() {
			Expect(capsOf()["named"]).To(Equal(graph.None))
		})
	})

	Context("when the directory holds no package", func() {
		It("reports the directory it could not load", func() {
			empty := GinkgoT().TempDir()

			_, _, err := graph.Detect(empty, nil, nil)

			Expect(err).To(MatchError(ContainSubstring(empty)))
		})
	})
})
