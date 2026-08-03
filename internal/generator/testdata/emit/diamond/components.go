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
