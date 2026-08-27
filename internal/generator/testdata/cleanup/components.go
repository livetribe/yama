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

package cleanup

import (
	"context"

	yama "l7e.io/yama"
)

// The var block asserts, at compile time, which lifecycle interfaces each
// component implements. Capability detection must reach the same answers the
// compiler does here.
var (
	_ yama.Stopper = (*Conn)(nil)
	_ yama.Starter = (*Cache)(nil)
	_ yama.Stopper = (*Worker)(nil)
	_ yama.Stopper = (*App)(nil)
)

// Pool's provider returns a cleanup and the value implements no lifecycle
// interface, so the cleanup is Pool's whole teardown.
type Pool struct{}

func NewPool() (*Pool, func()) {
	return &Pool{}, func() {}
}

// Plain implements no lifecycle interface and its provider returns no cleanup, so
// it occupies no level. It sits between Pool and Conn, which must stay ordered
// relative to each other through it.
type Plain struct{}

func NewPlain(*Pool) *Plain {
	return &Plain{}
}

// Conn's provider returns a cleanup and the value implements Stopper, so the two
// are one teardown participant: the cleanup runs, then the value's own Stop.
type Conn struct{}

func NewConn(*Plain) (*Conn, func()) {
	return &Conn{}, func() {}
}

func (*Conn) Stop(context.Context) {}

// Cache's provider returns a cleanup and the value's one capability is not
// Stopper, so the cleanup is its whole teardown while Start still runs.
type Cache struct{}

func NewCache(*Conn) (*Cache, func()) {
	return &Cache{}, func() {}
}

func (*Cache) Start(context.Context) error { return nil }

// Worker implements Stopper and its provider returns no cleanup, so its own Stop
// is its whole teardown.
type Worker struct{}

func NewWorker() *Worker {
	return &Worker{}
}

func (*Worker) Stop(context.Context) {}

// App is the graph's root. Its provider returns a cleanup, which folds at the
// root's own position like any other.
type App struct {
	cache  *Cache
	worker *Worker
}

func NewApp(cache *Cache, worker *Worker) (*App, func()) {
	return &App{cache: cache, worker: worker}, func() {}
}

func (*App) Stop(context.Context) {}
