// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package tagged is a dedicated fixture for cmd/yama's own CLI smoke test — kept
// separate from internal/generator's fixture of the same shape so the two
// packages' tests never touch the same directory. `go test ./...` runs packages
// concurrently, and two runs over one directory do not lock against each other:
// the second one's cleanup can delete a file the first one put back.
package tagged

// Root depends on Dep, which is defined only under the "special" build tag.
type Root struct{ d *Dep }

// NewRoot provides the Root.
func NewRoot(d *Dep) *Root { return &Root{d: d} }
