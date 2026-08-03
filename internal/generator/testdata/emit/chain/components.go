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
