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

// Package generator implements yama's build-time code generator: it runs
// Google Wire, walks the AST of the resulting wire_gen.go, computes
// lifecycle ordering, and emits lifecycle_gen.go into the target
// application package.
//
// See docs/adr/ADR-002-wire-as-graph-source.md and
// docs/adr/ADR-008-parse-generated-wire-output.md. This package must import
// only Google Wire's public types, never Google Wire's unexported
// implementation packages.
//
// Not yet implemented; see implementation_plan_claude.md Phases 5-9.
package generator
