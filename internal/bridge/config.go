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
