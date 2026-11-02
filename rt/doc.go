// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package rt is yama's runtime-support package: the generic lifecycle execution
// machinery that generated lifecycle_gen.go code imports and calls. Its symbols
// are exported so generated application-package code can reach them, but they are
// not part of yama's stable public API and carry no compatibility promise;
// applications use package yama, not this package, directly.
//
// The machinery here is graph-independent — identical in every application. The
// graph-specific part is the generated lifecycle constructor, which names the
// components and the order of the levels it adds.
package rt
