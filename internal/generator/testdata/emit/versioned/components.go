// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package versioned

import (
	"context"

	"l7e.io/yama/internal/generator/testdata/emit/versioned/v2"
)

// A depends on a type from a package the stub imports without an alias.
type A struct{ c *thing.Client }

func NewA(c *thing.Client) *A { return &A{c: c} }

func (*A) Start(context.Context) error { return nil }
