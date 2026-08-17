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

package wire_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

var _ = Describe("Diagnostics", func() {
	It("takes Yama's own prefix off the injector that Google Wire names", func() {
		logged := "wire: /app/lifecycle.go:14:1: inject yama_NewAppLifecycle: no provider found"

		Expect(wire.Diagnostics(logged)).To(ContainSubstring("inject NewAppLifecycle: no provider found"))
	})

	It("leaves out the line that names a file Google Wire wrote", func() {
		logged := "wire: example.com/app: wrote /app/wire_gen.go\nwire: generate failed"

		Expect(wire.Diagnostics(logged)).To(Equal("wire: generate failed"))
	})

	It("keeps every other line as it stands", func() {
		logged := "wire: /app/lifecycle.go:14:1: something went wrong\nwire: generate failed"

		Expect(wire.Diagnostics(logged)).To(Equal(logged))
	})

	It("keeps a path that carries the same prefix", func() {
		logged := "wire: /app/yama_thing/lifecycle.go:1:1: inject yama_NewApp: bad"

		diagnostic := wire.Diagnostics(logged)

		Expect(diagnostic).To(ContainSubstring("/app/yama_thing/lifecycle.go"))
		Expect(diagnostic).To(ContainSubstring("inject NewApp:"))
	})

	It("returns nothing for output that holds no line of its own", func() {
		Expect(wire.Diagnostics("")).To(BeEmpty())
	})
})

var _ = Describe("DropLineDirectives", func() {
	It("blanks a directive that starts a line", func() {
		src := []byte("package app\n\n//line /app/lifecycle.go:14:1\nfunc F() {}\n")

		Expect(string(wire.DropLineDirectives(src))).NotTo(ContainSubstring("//line "))
	})

	It("keeps every offset and every line number", func() {
		src := []byte("package app\n\n//line /app/lifecycle.go:14:1\nfunc F() {}\n")

		Expect(wire.DropLineDirectives(src)).To(HaveLen(len(src)))
	})

	It("leaves a directive that starts no line as it stands", func() {
		src := []byte("package app\n\nfunc F() {} //line /app/lifecycle.go:14:1\n")

		Expect(string(wire.DropLineDirectives(src))).To(ContainSubstring("//line /app/lifecycle.go:14:1"))
	})

	It("leaves the same text inside a string literal as it stands", func() {
		src := []byte("package app\n\nconst S = `\n//line /app/lifecycle.go:14:1\n`\n")

		Expect(string(wire.DropLineDirectives(src))).To(ContainSubstring("//line /app/lifecycle.go:14:1"))
	})

	It("leaves source that carries no directive as it stands", func() {
		src := []byte("package app\n\nfunc F() {}\n")

		Expect(string(wire.DropLineDirectives(src))).To(Equal(string(src)))
	})
})
