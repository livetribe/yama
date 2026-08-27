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

package hello

import (
	"context"
	"fmt"
	"io"

	"github.com/google/wire"

	yama "l7e.io/yama"
)

// GraphSet builds the whole graph from the io.Writer the lifecycle constructor
// takes as an argument.
var GraphSet = wire.NewSet(NewConfig, NewStore, NewServer)

// The capabilities each component carries. Config declares none.
var (
	_ yama.Starter = (*Store)(nil)
	_ yama.Stopper = (*Store)(nil)

	_ yama.Starter  = (*Server)(nil)
	_ yama.Quiescer = (*Server)(nil)
	_ yama.Stopper  = (*Server)(nil)
)

// Config is the application's configuration. It implements no lifecycle
// capability, so it is a dependency of the graph and not a member of any level.
type Config struct {
	Greeting string
}

// NewConfig provides the Config.
func NewConfig() Config {
	return Config{Greeting: "hello"}
}

// Store holds the application's data. Its provider returns a cleanup function,
// which runs at the store's own position in the teardown, before Stop.
type Store struct {
	w   io.Writer
	cfg Config
}

// NewStore provides the Store and the cleanup that releases what it holds.
func NewStore(w io.Writer, cfg Config) (store *Store, cleanup func()) {
	store = &Store{w: w, cfg: cfg}
	cleanup = func() {
		fmt.Fprintln(store.w, "store: released")
	}

	return store, cleanup
}

func (s *Store) Start(context.Context) error {
	fmt.Fprintln(s.w, "store: open")

	return nil
}

func (s *Store) Stop(context.Context) {
	fmt.Fprintln(s.w, "store: closed")
}

func (s *Store) String() string {
	return "store"
}

// Server serves requests from the Store. It depends on the Store, so it starts
// after it and stops before it.
type Server struct {
	w     io.Writer
	store *Store
}

// NewServer provides the Server.
func NewServer(w io.Writer, store *Store) *Server {
	return &Server{w: w, store: store}
}

func (s *Server) Start(context.Context) error {
	fmt.Fprintf(s.w, "server: serving %q\n", s.store.cfg.Greeting)

	return nil
}

// Quiesce stops accepting new work. It runs before any component's Stop, so the
// store is still open while the server drains.
func (s *Server) Quiesce(context.Context) {
	fmt.Fprintln(s.w, "server: draining")
}

func (s *Server) Stop(context.Context) {
	fmt.Fprintln(s.w, "server: stopped")
}

func (s *Server) String() string {
	return "server"
}
