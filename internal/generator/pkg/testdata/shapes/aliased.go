// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package shapes

import (
	gwire "github.com/google/wire"

	y "l7e.io/yama"
)

// NewAliased states Yama and Google Wire under a name of this file's own. An
// alias is per file, and the other stub file of this package states neither.
func NewAliased(opts ...y.Option) (*App, y.Lifecycle, error) {
	panic(gwire.Build(NewApp))
}
