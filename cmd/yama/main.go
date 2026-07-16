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

// Command yama runs Google Wire and yama's generator to produce
// wire_gen.go and lifecycle_gen.go for a target package.
//
// Not yet implemented; see implementation_plan_claude.md Phases 5-9.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "yama: generator command not yet implemented (see implementation_plan_claude.md Phase 5-9)")
	os.Exit(1)
}
