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

package structcomp

// A is a leaf provider value.
type A struct{}

// NewA provides an A.
func NewA() *A { return &A{} }

// B is provided together with a cleanup function, exercising cleanup detection on
// a value that is not the root.
type B struct{}

// NewB provides a B and its cleanup.
func NewB() (*B, func()) { return &B{}, func() {} }

// Cfg is assembled by wire.Struct and consumed downstream rather than returned.
type Cfg struct {
	A *A
	B *B
}

// Server consumes the wire.Struct-built Cfg.
type Server struct{ cfg *Cfg }

// NewServer provides a Server.
func NewServer(c *Cfg) *Server { return &Server{cfg: c} }

// Root is returned by a provider call rather than a struct literal.
type Root struct{ s *Server }

// NewRoot provides the Root.
func NewRoot(s *Server) *Root { return &Root{s: s} }
