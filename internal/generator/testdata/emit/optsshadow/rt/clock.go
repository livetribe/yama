// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package rt

// Clock is provided by a package whose name is the one Yama wants for the
// runtime-support package.
type Clock struct{}

func NewClock() *Clock { return &Clock{} }
