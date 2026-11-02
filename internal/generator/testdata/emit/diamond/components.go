// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package diamond

import "context"

// A diamond: Left and Right both depend on Base and neither depends on the
// other, so they share a level, and Top depends on both.
type Base struct{}

type Left struct{ base *Base }

type Right struct{ base *Base }

type Top struct {
	left  *Left
	right *Right
}

func NewBase() *Base { return &Base{} }

func NewLeft(base *Base) *Left { return &Left{base: base} }

func NewRight(base *Base) *Right { return &Right{base: base} }

func NewTop(left *Left, right *Right) *Top { return &Top{left: left, right: right} }

func (*Base) Start(context.Context) error { return nil }

func (*Left) Start(context.Context) error { return nil }

func (*Right) Start(context.Context) error { return nil }

func (*Top) Start(context.Context) error { return nil }
