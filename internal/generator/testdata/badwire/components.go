// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package badwire

// Base has no provider in the set below, so wire generation fails.
type Base struct{}

// Root needs a *Base.
type Root struct{}

// NewRoot requires a *Base that the provider set does not supply.
func NewRoot(b *Base) *Root { return &Root{} }
