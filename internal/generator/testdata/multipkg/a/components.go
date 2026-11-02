// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package a

// Dep is A's sole dependency.
type Dep struct{}

// NewDep provides a Dep.
func NewDep() *Dep { return &Dep{} }

// Root is a's returned root.
type Root struct{ d *Dep }

// NewRoot provides the Root.
func NewRoot(d *Dep) *Root { return &Root{d: d} }
