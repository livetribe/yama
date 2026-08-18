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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/pkg"
)

// optionsStub is a stub that forwards options under the name that it states.
func optionsStub(params ...pkg.Field) pkg.Stub {
	return pkg.Stub{
		Name:    "NewAppLifecycle",
		Params:  append(params, pkg.Field{Name: "opts", Type: "...yama.Option"}),
		Results: []pkg.Field{{Type: "*App"}, {Type: "yama.Lifecycle"}, {Type: "error"}},
		HasOpts: true,
		Yama:    "yama",
	}
}

// The lifecycle file shares a block with the package that contains it. Each
// constructor shares a scope with the parameters that the stub declares. Yama's
// own two imports take a name that neither the block nor a scope holds.
var _ = Describe("nameFile", func() {
	Context("a stub that declares a parameter of Yama's own names", func() {
		It("gives the runtime import another name", func() {
			stubs := []pkg.Stub{optionsStub(pkg.Field{Name: "rt", Type: "*Registry"})}

			names := nameFile(nil, nil, nil, stubs)

			Expect(names.rt).NotTo(Equal("rt"))
		})

		It("gives the public import another name", func() {
			stubs := []pkg.Stub{optionsStub(pkg.Field{Name: "yama", Type: "*Thing"})}

			names := nameFile(nil, nil, nil, stubs)

			Expect(names.yama).NotTo(Equal("yama"))
		})

		It("keeps a parameter that neither import needs", func() {
			stubs := []pkg.Stub{optionsStub(pkg.Field{Name: "ctx", Type: "context.Context"})}

			names := nameFile(nil, nil, nil, stubs)

			Expect(names.rt).To(Equal("rt"))
			Expect(names.yama).To(Equal("yama"))
		})
	})

	// Google Wire names each value that it builds after the type that built it.
	// The constructor body states those names beside its own parameters.
	Context("an options parameter that a Google Wire component also names", func() {
		It("forwards the options under another name", func() {
			injectors := []graph.Injector{{
				Name:       "yama_NewAppLifecycle",
				Components: []graph.Component{{Name: "opts"}},
			}}

			names := nameFile(nil, injectors, nil, []pkg.Stub{optionsStub()})

			Expect(names.opts).To(HaveLen(1))
			Expect(names.opts[0]).NotTo(Equal("opts"))
		})
	})

	// One constructor's options parameter shares a scope with no other
	// constructor's options parameter.
	Context("two stubs that each forward options", func() {
		It("gives both the name that each one states", func() {
			stub := optionsStub()

			names := nameFile(nil, nil, nil, []pkg.Stub{stub, stub})

			Expect(names.opts).To(Equal([]string{"opts", "opts"}))
		})
	})
})

// The lifecycle file drops the stub file's import of Yama's runtime package and
// states one of its own. Every type that names that package takes the new name.
var _ = Describe("signature", func() {
	It("gives the runtime package the name that the lifecycle file states", func() {
		stub := optionsStub(pkg.Field{Name: "b", Type: "*runtime.Builder"})
		names := naming{yama: "yama", rt: "rt", opts: []string{"opts"}}

		rendered := signature(&stub, "opts", &names, "runtime")

		Expect(rendered).To(ContainSubstring("*rt.Builder"))
		Expect(rendered).NotTo(ContainSubstring("runtime.Builder"))
	})

	It("leaves a type alone when the stub named no runtime import", func() {
		stub := optionsStub(pkg.Field{Name: "cfg", Type: "*Config"})
		names := naming{yama: "yama", rt: "rt", opts: []string{"opts"}}

		rendered := signature(&stub, "opts", &names, "")

		Expect(rendered).To(ContainSubstring("cfg *Config"))
	})
})
