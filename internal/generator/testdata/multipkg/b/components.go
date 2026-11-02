// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package b

// Svc is b's sole dependency.
type Svc struct{}

// NewSvc provides a Svc.
func NewSvc() *Svc { return &Svc{} }

// Root is b's returned root.
type Root struct{ s *Svc }

// NewRoot provides the Root.
func NewRoot(s *Svc) *Root { return &Root{s: s} }
