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

package fanout

import "context"

// A wide fan-out: four leaves depend on one base and on nothing else, so all
// four share the second level.
type Base struct{}

type Leaf1 struct{ base *Base }

type Leaf2 struct{ base *Base }

type Leaf3 struct{ base *Base }

type Leaf4 struct{ base *Base }

type Hub struct {
	Leaf1 *Leaf1
	Leaf2 *Leaf2
	Leaf3 *Leaf3
	Leaf4 *Leaf4
}

func NewBase() *Base { return &Base{} }

func NewLeaf1(base *Base) *Leaf1 { return &Leaf1{base: base} }

func NewLeaf2(base *Base) *Leaf2 { return &Leaf2{base: base} }

func NewLeaf3(base *Base) *Leaf3 { return &Leaf3{base: base} }

func NewLeaf4(base *Base) *Leaf4 { return &Leaf4{base: base} }

func (*Base) Start(context.Context) error { return nil }

func (*Leaf1) Start(context.Context) error { return nil }

func (*Leaf2) Start(context.Context) error { return nil }

func (*Leaf3) Start(context.Context) error { return nil }

func (*Leaf4) Start(context.Context) error { return nil }
