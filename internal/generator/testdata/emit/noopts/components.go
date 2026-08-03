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

package noopts

import "context"

// Config is supplied by the caller, so it reaches the graph as a parameter
// rather than through a provider.
type Config struct{ Name string }

// App is the graph's one component.
type App struct{ cfg *Config }

func NewApp(cfg *Config) *App { return &App{cfg: cfg} }

func (*App) Start(context.Context) error { return nil }

func (*App) Stop(context.Context) {}
