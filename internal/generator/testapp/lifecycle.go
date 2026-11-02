// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build yamainject

package testapp

import (
	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph GraphSet builds. The recorder collects
// what the components report, and the fault selects the component whose Start
// fails.
func NewLifecycle(rec *Recorder, fault Fault, opts ...yama.Option) (*Top, yama.Lifecycle, error) {
	panic(wire.Build(GraphSet))
}
