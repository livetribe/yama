// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package yama

import (
	"context"

	"l7e.io/yama/internal/bridge"
)

// FromContext returns the lifecycle component Yama is currently invoking, and
// true, when called from inside an interceptor chain. When no component is
// attached, or it is not a T, it returns the zero T and false.
//
// Instantiate T with the component's concrete type. An interceptor that is not
// scoped to a single component uses any and type-switches.
//
// For a printable identity, implement fmt.Stringer on the component; %T gives its
// type otherwise.
func FromContext[T any](ctx context.Context) (T, bool) {
	return bridge.FromContext[T](ctx)
}
