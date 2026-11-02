// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package testapp is a generation target whose committed lifecycle_gen.go the
// behavioral contract runs against. Its components record every lifecycle call
// they receive, so a spec can assert the ordering, the failure handling, and the
// teardown that the generated constructor and the runtime-support package
// produce together.
//
// Yama emits lifecycle_gen.go here. Edit the stub in lifecycle.go and
// regenerate; do not edit the generated file.
package testapp
