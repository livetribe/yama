// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package optscollision

import "context"

// A and B are two graphs in one package, so both constructors declare their
// options parameter in one file scope.
type A struct{}

type B struct{}

func NewA() *A { return &A{} }

func NewB() *B { return &B{} }

func (*A) Start(context.Context) error { return nil }

func (*B) Start(context.Context) error { return nil }
