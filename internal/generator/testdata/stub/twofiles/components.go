// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package twofiles

// Two roots, each declared by a stub in its own file.
type First struct{}

type Second struct{}

func NewFirst() *First { return &First{} }

func NewSecond() *Second { return &Second{} }
