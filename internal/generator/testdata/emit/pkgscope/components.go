// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package pkgscope

import "context"

// rt is declared in the package block. The emitted file shares that block, and
// Go forbids one identifier in both the file block and the package block, so
// the emitted file cannot import anything under this name.
type rt struct{}

type A struct{ r rt }

func NewA() *A { return &A{} }

func (*A) Start(context.Context) error { return nil }
