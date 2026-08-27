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
