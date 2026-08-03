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
