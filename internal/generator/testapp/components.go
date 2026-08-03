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

package testapp

import (
	"context"
	"errors"
	"fmt"

	yama "l7e.io/yama/v2"
)

var (
	_ yama.Starter  = (*Base)(nil)
	_ yama.Quiescer = (*Base)(nil)
	_ yama.Stopper  = (*Base)(nil)
)

// ErrStartFault is what a component's Start returns when the graph was built to
// fail at that component.
var ErrStartFault = errors.New("testapp: start fault")

// Fault names the component whose Start fails, and is empty for a graph that
// starts cleanly. It reaches the providers as an argument to the lifecycle
// constructor.
type Fault struct {
	FailStart string
}

// Probe is the recording behavior every lifecycle component in this graph
// embeds. It implements all three capabilities and reports each call under the
// component's own name.
type Probe struct {
	rec   *Recorder
	name  string
	fault Fault
}

// NewProbe returns a probe that records under name.
func NewProbe(rec *Recorder, name string, fault Fault) *Probe {
	return &Probe{rec: rec, name: name, fault: fault}
}

func (p *Probe) Start(context.Context) error {
	p.rec.Record("start " + p.name)
	if p.fault.FailStart == p.name {
		return fmt.Errorf("%w: %s", ErrStartFault, p.name)
	}

	return nil
}

func (p *Probe) Quiesce(context.Context) {
	p.rec.Record("quiesce " + p.name)
}

func (p *Probe) Stop(context.Context) {
	p.rec.Record("stop " + p.name)
}

func (p *Probe) String() string {
	return p.name
}

// Res implements no lifecycle capability, and its provider returns a cleanup, so
// it occupies a level for that cleanup alone.
type Res struct{}

// Base is the graph's first lifecycle component.
type Base struct {
	*Probe

	res *Res
}

// Mid depends on Base and shares its level with Leaf.
type Mid struct {
	*Probe

	base *Base
}

// Leaf depends on Base, and its provider returns a cleanup, so it is a
// component and a cleanup at one position.
type Leaf struct {
	*Probe

	base *Base
}

// Top depends on both members of the level below it.
type Top struct {
	*Probe

	mid  *Mid
	leaf *Leaf
}

func NewRes(rec *Recorder) (res *Res, cleanup func()) {
	cleanup = func() { rec.Record("cleanup res") }

	return &Res{}, cleanup
}

func NewBase(rec *Recorder, fault Fault, res *Res) *Base {
	return &Base{Probe: NewProbe(rec, "base", fault), res: res}
}

func NewMid(rec *Recorder, fault Fault, base *Base) *Mid {
	return &Mid{Probe: NewProbe(rec, "mid", fault), base: base}
}

func NewLeaf(rec *Recorder, fault Fault, base *Base) (leaf *Leaf, cleanup func()) {
	cleanup = func() { rec.Record("cleanup leaf") }

	return &Leaf{Probe: NewProbe(rec, "leaf", fault), base: base}, cleanup
}

func NewTop(rec *Recorder, fault Fault, mid *Mid, leaf *Leaf) *Top {
	return &Top{Probe: NewProbe(rec, "top", fault), mid: mid, leaf: leaf}
}
