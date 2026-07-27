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
// The front-end is implemented: a Generator, built from Options with
// NewGenerator, runs Google Wire over one or more packages and parses each
// injector into a graph of creation events, dependency edges, results, and
// cleanup functions. Wire's wire_gen.go is transient — a run leaves the
// directory as it found it, preserving a wire_gen.go it did not create and
// removing the one it did.
//
// The parser is coupled to the shape of Google Wire's generated output and is
// validated against Google Wire v0.7.0, pinned in go.mod. A version bump can
// change that shape; the drift check is the guard.
package generator
