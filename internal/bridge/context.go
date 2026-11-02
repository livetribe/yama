// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package bridge

import "context"

// componentKey is the context key for the component identity.
type componentKey struct{}

// WithComponent returns a copy of ctx carrying the component identity c. The
// runtime-support per-component wrapper calls this before invoking an interceptor
// chain so that interceptors can recover the identity with
// yama.FromContext.
func WithComponent(ctx context.Context, c any) context.Context {
	return context.WithValue(ctx, componentKey{}, c)
}

// FromContext returns the identity previously attached with WithComponent, and true,
// when one is present and is a T. Otherwise it returns the zero T and false.
func FromContext[T any](ctx context.Context) (T, bool) {
	v, ok := ctx.Value(componentKey{}).(T)
	return v, ok
}
