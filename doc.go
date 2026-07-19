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

// Package yama is the runtime library for yama v2, a compile-time lifecycle
// orchestration framework generated from Google Wire dependency graphs.
//
// A component joins the lifecycle by implementing any of Starter, Quiescer, and
// Stopper; components that implement none still shape ordering as dependencies.
// Yama's generator reads Wire's generated injector and fills in the body of a
// lifecycle constructor the application declared, which returns the application
// together with its Lifecycle:
//
//	app, lifecycle, err := NewLifecycle(WithInterceptors(i1, i2), WithBeginComponents(c1))
//
// Start brings the graph up in dependency order. Stop takes it down in reverse,
// quiescing every component before any teardown begins. Logging, metrics, and
// tracing are not framework features — they attach as interceptors.
package yama
