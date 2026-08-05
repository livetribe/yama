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

package value

import "context"

// Banner is the value the stub supplies as a literal, so the emitted file must
// carry that literal instead of the variable Wire bound it to.
type Banner string

// App is the graph root, and it takes the value from a provider call.
type App struct{ banner Banner }

func NewApp(banner Banner) *App { return &App{banner: banner} }

func (*App) Start(context.Context) error { return nil }

func (*App) Stop(context.Context) {}
