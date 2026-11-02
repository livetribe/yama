// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package exec runs the start, quiesce, and stop passes over a lifecycle's
// levels. A Level runs its members concurrently. It reports the members'
// failures as one combined failure. A lifecycle runs its levels in order.
// When Start fails, the lifecycle runs Stop on what Start already reached.
//
// NewChains builds Chains, the per-operation interceptor chains. NewChains
// puts the built-in overrun interceptor innermost on every chain.
// WrapComponent binds a component's operations through these chains.
// A Cleanup adapts a Google Wire cleanup function to the same
// CompleteLifecycle interface that a component satisfies. A Level can hold
// a Cleanup as a member with no special case.
//
// Start and Stop recover a component's panic. They log the panic. They
// treat the panic as that operation's failure. A Level's spawn method does
// that recovery.
package exec
