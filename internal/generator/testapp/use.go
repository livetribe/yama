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

package testapp

import yama "l7e.io/yama"

// Start calls the constructor this package's stub declares, and returns what it
// returns. This file carries no build tag.
func Start(rec *Recorder, fault Fault, opts ...yama.Option) (*Top, yama.Lifecycle, error) {
	return NewLifecycle(rec, fault, opts...)
}
