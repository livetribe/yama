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
