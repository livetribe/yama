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
