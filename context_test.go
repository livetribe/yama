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

package yama_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yama "l7e.io/yama"
	"l7e.io/yama/internal/bridge"
)

// component is a stand-in application component. It implements Starter, which is
// what makes it a lifecycle component.
type component struct{ name string }

func (c *component) Start(context.Context) error { return nil }

// router is a second component type, for the mismatched-type case.
type router struct{}

func (r *router) Start(context.Context) error { return nil }

// TestComponentRoundTripThroughYama guards against yama.FromContext ever
// growing its own context key instead of delegating to bridge's.
func TestComponentRoundTripThroughYama(t *testing.T) {
	want := &component{name: "kafka-consumer"}

	ctx := bridge.WithComponent(context.Background(), want)

	got, ok := yama.FromContext[*component](ctx)
	require.True(t, ok, "expected FromContext to recover the attached component")
	assert.Same(t, want, got, "component did not round-trip")
}

// TestFromContextWrongType covers the other half of FromContext's documented
// contract: a component attached under one concrete type yields (zero, false)
// when read back as a different one, not a false positive.
func TestFromContextWrongType(t *testing.T) {
	ctx := bridge.WithComponent(context.Background(), &component{name: "kafka-consumer"})

	got, ok := yama.FromContext[*router](ctx)
	assert.False(t, ok, "expected ok=false for a mismatched type, got %v", got)
}

// TestFromContextAbsent covers the remaining half: nothing attached yields
// false, not a panic.
func TestFromContextAbsent(t *testing.T) {
	got, ok := yama.FromContext[*component](context.Background())
	assert.False(t, ok, "expected ok=false on a bare context, got ok=true with %+v", got)
}
