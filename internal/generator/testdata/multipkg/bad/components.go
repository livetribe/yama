// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package bad

// Base has no provider in the set below, so wire generation fails: this
// package exists to prove a failure here does not stop a and b from
// generating in the same sweep.
type Base struct{}

// Root needs a *Base.
type Root struct{}

// NewRoot requires a *Base that the provider set does not supply.
func NewRoot(b *Base) *Root { return &Root{} }
