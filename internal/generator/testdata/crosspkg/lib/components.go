// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package lib

import "context"

// Dep is the graph's sole component. It implements Stopper.
type Dep struct{}

// NewDep provides a Dep.
func NewDep() *Dep { return &Dep{} }

func (*Dep) Stop(context.Context) {}

// Root is the value the graph builds.
type Root struct{ d *Dep }

// NewRoot provides the Root.
func NewRoot(d *Dep) *Root { return &Root{d: d} }
