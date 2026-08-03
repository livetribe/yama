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

package collision

import "context"

// Type names Google Wire turns into the locals rt and yama, which are the two
// names Yama wants for its own imports.
type Rt struct{}

type Yama struct{ rt *Rt }

type App struct{ yama *Yama }

func NewRt() *Rt { return &Rt{} }

func NewYama(rt *Rt) *Yama { return &Yama{rt: rt} }

func NewApp(yama *Yama) *App { return &App{yama: yama} }

func (*Rt) Start(context.Context) error { return nil }

func (*Yama) Start(context.Context) error { return nil }

func (*App) Start(context.Context) error { return nil }
