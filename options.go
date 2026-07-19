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

package yama

import "l7e.io/yama/v2/internal/bridge"

// Option is a construction-time input to the generated lifecycle constructor.
// It cannot be implemented outside Yama: doing so means naming *bridge.Config,
// which the internal/ rule forbids outside this module.
type Option interface {
	Apply(*bridge.Config)
}

type optionFunc func(*bridge.Config)

func (f optionFunc) Apply(c *bridge.Config) { f(c) }

// WithBeginComponents registers components as begin boundary components: each
// joins every lifecycle pass and runs before the pass's graph components, with
// no ordering relative to them. WithBeginComponents is variadic, accumulating
// registered components across calls.
func WithBeginComponents(components ...any) Option {
	return optionFunc(func(c *bridge.Config) {
		c.BeginComponents = append(c.BeginComponents, components...)
	})
}

// WithEndComponents registers components as end boundary components: each
// joins every lifecycle pass and runs after the pass's graph components, with
// no ordering relative to them. WithEndComponents is variadic, accumulating
// registered components across calls.
func WithEndComponents(components ...any) Option {
	return optionFunc(func(c *bridge.Config) {
		c.EndComponents = append(c.EndComponents, components...)
	})
}

// WithInterceptors registers interceptors: pass values implementing one or more
// of StartInterceptor, QuiesceInterceptor, and StopInterceptor. Each joins the
// chain for every operation whose interceptor interface it implements and runs
// for every component in that pass, with no per-component scoping.
// WithInterceptors is variadic, accumulating across calls; interceptors run in
// registration order.
func WithInterceptors(interceptors ...any) Option {
	return optionFunc(func(c *bridge.Config) {
		c.Interceptors = append(c.Interceptors, interceptors...)
	})
}
