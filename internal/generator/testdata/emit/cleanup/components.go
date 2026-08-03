// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
