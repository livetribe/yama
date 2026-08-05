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

package app

import "context"

// Svc is the graph's sole component. It implements Stopper.
type Svc struct{}

// NewSvc provides a Svc.
func NewSvc() *Svc { return &Svc{} }

func (*Svc) Stop(context.Context) {}

// App is the value the graph builds.
type App struct{ s *Svc }

// NewApp provides the App.
func NewApp(s *Svc) *App { return &App{s: s} }
