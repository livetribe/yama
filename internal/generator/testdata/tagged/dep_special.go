// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build special

package tagged

// Dep exists only when the "special" build tag is set.
type Dep struct{}

// NewDep provides a Dep.
func NewDep() *Dep { return &Dep{} }
