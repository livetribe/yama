// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package mixed

import "context"

// A is the whole graph, so the interesting part of this fixture is what sits
// beside it in wire.go rather than the graph itself.
type A struct{}

func NewA() *A { return &A{} }

func (*A) Start(context.Context) error { return nil }
