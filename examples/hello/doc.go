// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package hello is a small Yama application, in its own Go module.
//
// components.go declares the graph. lifecycle.go declares one stub for the
// constructor the application calls. lifecycle_gen.go is what Yama emits from
// them, and is the only generated file this package commits.
//
// The graph is four components:
//
//	server → client → store → config
//
// Config carries no lifecycle capability, so it is a dependency alone. Store
// starts and stops, and its provider returns a cleanup function. Client
// declares Close alone, and the closer directive on its provider binds that
// Close to the teardown pass. Server starts, quiesces, and stops.
//
// lifecycle_gen.go carries its own go:generate directive, so `go generate ./...`
// regenerates it without a directive here.
package hello
