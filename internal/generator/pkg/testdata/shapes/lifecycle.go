// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package shapes

import (
	"database/sql"

	"github.com/google/wire"

	yama "l7e.io/yama"
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
