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

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/bridge"
)

// startLogger is a minimal StartInterceptor, standing in for a real interceptor
// value passed to WithInterceptors.
type startLogger struct{ name string }

func (l *startLogger) Start(ctx context.Context, next yama.Starter) error {
	return next.Start(ctx)
}

// TestOptionsApplyFromAnotherPackage lives in yama_test, a separate package,
// standing in for the runtime-support package: it stops compiling if Apply is
// ever unexported.
func TestOptionsApplyFromAnotherPackage(t *testing.T) {
	begin, end := &component{name: "begin"}, &component{name: "end"}
	interceptor := &startLogger{name: "start-logger"}

	cfg := &bridge.Config{}
	for _, opt := range []yama.Option{
		yama.WithBeginComponents(begin),
		yama.WithEndComponents(end),
		yama.WithInterceptors(interceptor),
	} {
		opt.Apply(cfg)
	}

	if assert.Len(t, cfg.BeginComponents, 1, "WithBeginComponents did not record the component: %+v", cfg.BeginComponents) {
		assert.Same(t, begin, cfg.BeginComponents[0])
	}
	if assert.Len(t, cfg.EndComponents, 1, "WithEndComponents did not record the component: %+v", cfg.EndComponents) {
		assert.Same(t, end, cfg.EndComponents[0])
	}
	if assert.Len(t, cfg.Interceptors, 1, "WithInterceptors did not record the interceptor: %+v", cfg.Interceptors) {
		assert.Same(t, interceptor, cfg.Interceptors[0])
	}
}

// TestOptionsAccumulate proves the options are additive rather than last-one-wins:
// a caller may register several boundary components, across one or more calls,
// and each pass's set holds all of them.
func TestOptionsAccumulate(t *testing.T) {
	first, second, third := &component{name: "first"}, &component{name: "second"}, &component{name: "third"}

	cfg := &bridge.Config{}
	yama.WithBeginComponents(first).Apply(cfg)
	yama.WithBeginComponents(second, third).Apply(cfg)
	yama.WithEndComponents(first).Apply(cfg)
	yama.WithEndComponents(second, third).Apply(cfg)

	require.Len(t, cfg.BeginComponents, 3, "expected all begin components to accumulate, got %+v", cfg.BeginComponents)
	assert.Same(t, first, cfg.BeginComponents[0])
	assert.Same(t, second, cfg.BeginComponents[1])
	assert.Same(t, third, cfg.BeginComponents[2])

	require.Len(t, cfg.EndComponents, 3, "expected all end components to accumulate, got %+v", cfg.EndComponents)
	assert.Same(t, first, cfg.EndComponents[0])
	assert.Same(t, second, cfg.EndComponents[1])
	assert.Same(t, third, cfg.EndComponents[2])

	// Interceptors accumulate in registration order across calls; that order is
	// the interceptor chain's execution order.
	l1, l2, l3 := &startLogger{name: "l1"}, &startLogger{name: "l2"}, &startLogger{name: "l3"}
	yama.WithInterceptors(l1).Apply(cfg)
	yama.WithInterceptors(l2, l3).Apply(cfg)

	require.Len(t, cfg.Interceptors, 3, "expected all interceptors to accumulate, got %+v", cfg.Interceptors)
	assert.Same(t, l1, cfg.Interceptors[0])
	assert.Same(t, l2, cfg.Interceptors[1])
	assert.Same(t, l3, cfg.Interceptors[2])
}
