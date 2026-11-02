// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
