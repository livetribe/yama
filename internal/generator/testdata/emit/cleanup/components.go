// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import "context"

// Three cleanup shapes: a value with a cleanup and no capability of its own, a
// value with both, and a provider that returns a cleanup beside an error.
type Plain struct{}

type Capable struct{ plain *Plain }

type Failing struct{ capable *Capable }

type Root struct{ failing *Failing }

func NewPlain() (*Plain, func()) { return &Plain{}, func() {} }

func NewCapable(plain *Plain) (*Capable, func()) { return &Capable{plain: plain}, func() {} }

func NewFailing(capable *Capable) (*Failing, func(), error) {
	return &Failing{capable: capable}, func() {}, nil
}

func NewRoot(failing *Failing) *Root { return &Root{failing: failing} }

func (*Capable) Stop(context.Context) {}

func (*Failing) Start(context.Context) error { return nil }

func (*Root) Start(context.Context) error { return nil }
