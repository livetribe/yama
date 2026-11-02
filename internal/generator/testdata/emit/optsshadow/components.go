// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package optsshadow

import (
	"context"

	"github.com/google/wire"

	"l7e.io/yama/internal/generator/testdata/emit/optsshadow/rt"
)

// AppSet reaches rt.NewClock, so the re-emitted body refers to rt even though
// the stub never writes that name itself.
var AppSet = wire.NewSet(rt.NewClock, NewA)

type A struct{ c *rt.Clock }

func NewA(c *rt.Clock) *A { return &A{c: c} }

func (*A) Start(context.Context) error { return nil }
