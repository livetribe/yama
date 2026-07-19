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

// Command externalmod must NOT compile. It stands in for generated application
// code, which lives in a module separate from l7e.io/yama/v2, and it attempts the
// one thing Go's internal/ import-path rule forbids from outside the module:
// importing l7e.io/yama/v2/internal/bridge directly. The expected failure is
// Go's own "use of internal package ... not allowed" error. Real external code
// reaches Lifecycle construction only through the runtime-support package's
// exported API, never through internal/bridge.
package main

import (
	_ "l7e.io/yama/v2/internal/bridge"
)

func main() {}
