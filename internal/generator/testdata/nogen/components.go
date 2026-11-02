// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package nogen

// Dep is the single dependency of Root.
type Dep struct{}

// NewDep provides a Dep.
func NewDep() *Dep { return &Dep{} }

// Root is the injector's returned root.
type Root struct{ d *Dep }

// NewRoot provides the Root.
func NewRoot(d *Dep) *Root { return &Root{d: d} }
