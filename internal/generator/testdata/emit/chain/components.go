// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package chain

import "context"

// A chain of three components, each depending on the one before it, so every
// component lands in a level of its own.
type A struct{}

type B struct{ a *A }

type C struct{ b *B }

func NewA() *A { return &A{} }

func NewB(a *A) *B { return &B{a: a} }

func NewC(b *B) *C { return &C{b: b} }

func (*A) Start(context.Context) error { return nil }

func (*A) Stop(context.Context) {}

func (*B) Start(context.Context) error { return nil }

func (*B) Stop(context.Context) {}

func (*C) Start(context.Context) error { return nil }

func (*C) Stop(context.Context) {}
