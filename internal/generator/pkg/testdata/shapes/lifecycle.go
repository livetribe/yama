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

//go:build yamainject

package shapes

import (
	"database/sql"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewPlain forwards no options. Every parameter it declares is a graph
// parameter.
func NewPlain(db *sql.DB) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}

// NewNamedOptions binds its options parameter to a name of its own.
func NewNamedOptions(db *sql.DB, settings ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}

// NewUnnamedOptions binds no name to any parameter it declares.
func NewUnnamedOptions(*sql.DB, ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}

// NewBlankOptions binds its options parameter to the blank identifier.
func NewBlankOptions(db *sql.DB, _ ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}

// NewNoParams declares no parameter at all.
func NewNoParams() (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp))
}
