// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
