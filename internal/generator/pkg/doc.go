// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package pkg states what a run knows about one target package: the lifecycle
// stubs that it declares, the entries of its import block, and the name of each
// of those entries. It reads those facts as well. Load reads the stub files of
// a directory. ImportPath and Names ask the Go toolchain for what no file
// states.
//
// An Info holds its own fields. Every write to a field goes through a method of
// its own, so no caller can store a name without the check that the name needs.
// Stub, Field, and Import are records that a caller states outright.
//
// The files of this package divide by concern. A file with no suffix holds a
// type and the views that every reader takes. A file with a suffix holds the
// views that one reader takes.
package pkg
