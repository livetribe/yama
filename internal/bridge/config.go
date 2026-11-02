// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package bridge

// Config accumulates the construction-time inputs that yama.Option values apply.
// The runtime-support package reads it when assembling a lifecycle.
type Config struct {
	// BeginComponents are boundary components at the graph's base extreme: they
	// start before every graph component and tear down after every graph component.
	BeginComponents []any
	// EndComponents are boundary components at the graph's top extreme: they start
	// after every graph component and tear down before every graph component.
	EndComponents []any
	// Interceptors are the globally-attached interceptors, in registration
	// order. Each joins an operation's chain when it implements that
	// operation's interceptor interface.
	Interceptors []any
}
