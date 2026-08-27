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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/internal/generator/graph"
)

// parseOne reads one injector out of a body that states it.
func parseOne(body string) graph.Injector {
	GinkgoHelper()

	injectors, err := graph.Parse([]byte(body), []string{"yama_NewApp"})

	Expect(err).NotTo(HaveOccurred())
	Expect(injectors).To(HaveLen(1))

	return injectors[0]
}

// Google Wire binds every error that it checks to one name of its own. It takes
// another name when the target package block already holds "err". A cleanup is
// a name that no error check names.
var _ = Describe("Parse, over the names that one statement binds", func() {
	Context("a provider that returns an error under a name of Google Wire's own", func() {
		It("reads that name as the error, and the component carries no cleanup", func() {
			injector := parseOne(`package app

func yama_NewApp() (*App, func(), error) {
	dep, err2 := NewDep()
	if err2 != nil {
		return nil, nil, err2
	}
	return dep, func() {
	}, nil
}
`)

			Expect(injector.Components).To(HaveLen(1))
			Expect(injector.Components[0].Name).To(Equal("dep"))
			Expect(injector.Components[0].Cleanup).To(BeEmpty())
		})
	})

	Context("a provider that returns a cleanup", func() {
		It("names the cleanup after the component that it cleans up", func() {
			injector := parseOne(`package app

func yama_NewApp() (*App, func(), error) {
	dep, cleanup, err := NewDep()
	if err != nil {
		return nil, nil, err
	}
	return dep, func() {
		cleanup()
	}, nil
}
`)

			Expect(injector.Components).To(HaveLen(1))
			Expect(injector.Components[0].Cleanup).To(Equal("depCleanup"))
		})
	})
})

// renameCleanups gives each cleanup the name of the component that it cleans
// up. A struct literal can key a field by that same name, and a field name is
// not a value.
var _ = Describe("Parse, over a struct literal that keys a field by a cleanup's name", func() {
	It("renames the value and leaves the field key alone", func() {
		injector := parseOne(`package app

func yama_NewApp() (*App, func(), error) {
	dep, cleanup, err := NewDep()
	if err != nil {
		return nil, nil, err
	}
	app := &App{cleanup: dep}
	return app, func() {
		cleanup()
	}, nil
}
`)

		body := strings.Join(injector.Statements, "\n")

		Expect(body).To(ContainSubstring("&App{cleanup: dep}"))
		Expect(body).NotTo(ContainSubstring("depCleanup: dep"))
		Expect(body).To(ContainSubstring("dep, depCleanup, err := NewDep()"))
	})
})

// A cleanup takes the name of the component that it cleans up. Another
// component can already hold that name. The cleanup then takes a number.
var _ = Describe("Parse, over a component that holds a cleanup's name", func() {
	It("gives the cleanup a name that no component holds", func() {
		injector := parseOne(`package app

func yama_NewApp() (*App, func(), error) {
	depCleanup := NewOther()
	dep, cleanup, err := NewDep()
	if err != nil {
		return nil, nil, err
	}
	return dep, func() {
		cleanup()
	}, nil
}
`)

		Expect(injector.Components).To(HaveLen(2))
		Expect(injector.Components[0].Name).To(Equal("depCleanup"))
		Expect(injector.Components[1].Cleanup).To(Equal("depCleanup2"))

		body := strings.Join(injector.Statements, "\n")

		Expect(body).To(ContainSubstring("dep, depCleanup2, err := NewDep()"))
	})
})
