// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
