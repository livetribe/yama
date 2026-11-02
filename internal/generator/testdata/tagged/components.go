// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package tagged

// Root depends on Dep, which is defined only under the "special" build tag —
// this fixture proves Options.Tags reaches both the wire invocation (so it can
// resolve NewDep) and Yama's own package load (so it can resolve NewDep's type
// too, since wire_gen.go references it by name).
type Root struct{ d *Dep }

// NewRoot provides the Root.
func NewRoot(d *Dep) *Root { return &Root{d: d} }
