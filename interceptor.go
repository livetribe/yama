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

package yama

import "context"

// Interceptors are operation-specific: a start interceptor participates only in
// the start chain, a quiesce interceptor only in the quiesce chain, and a stop
// interceptor only in the stop chain. A single type may implement any combination
// of the three. The operation is conveyed by which method ran, so no operation
// identity is carried in context.

// StartInterceptor wraps the start notification of a lifecycle component. It
// may invoke next.Start to proceed, modify the context first, alter the returned
// error, or return without calling next to suppress the start.
type StartInterceptor interface {
	Start(ctx context.Context, next Starter) error
}

// QuiesceInterceptor wraps the quiesce notification of a lifecycle component.
// It may invoke next.Quiesce to proceed, modify the context first, or return
// without calling next to suppress the quiesce notification.
type QuiesceInterceptor interface {
	Quiesce(ctx context.Context, next Quiescer)
}

// StopInterceptor wraps the teardown notification of a lifecycle component. It
// may invoke next.Stop to proceed, modify the context first, or return without
// calling next to suppress the teardown.
type StopInterceptor interface {
	Stop(ctx context.Context, next Stopper)
}
