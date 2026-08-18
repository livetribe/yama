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

package pkg

// blankName binds nothing. A parameter that carries it states no name that the
// constructor body can reach.
const blankName = "_"

// A Stub is one lifecycle constructor that a person declared. Doc carries no
// comment markers. Providers holds one entry for each argument of the
// wire.Build call, printed as the person wrote it.
type Stub struct {
	Name      string
	Doc       string
	Params    []Field
	Results   []Field
	Providers []string
	HasOpts   bool

	// Yama is the name that the stub's own file uses for Yama's public
	// package. The stub's signature names its Option and its Lifecycle
	// through this name.
	Yama string

	// Wire is the name that the stub's own file uses for Google Wire. The
	// stub's body states its provider list through a Build call on this name.
	Wire string

	// File names the file that declares the stub. Line and Column are the
	// position of the declaration in that file. A file that Yama derives from
	// the stub carries this position, so the Go toolchain reports the stub
	// rather than the file that Yama wrote.
	File   string
	Line   int
	Column int
}

// A Field is one parameter or one result. Name is empty when the declaration
// gave none.
type Field struct {
	Name string
	Type string
}

// YamaName returns the name that the stub's own file uses for Yama's public
// package. A stub that states no name takes the name that the path itself
// states.
func (s *Stub) YamaName() string {
	return PackageName(s.Yama, PackagePath)
}
