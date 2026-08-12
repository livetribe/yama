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

// Package generator implements yama's build-time code generator. It runs
// Google Wire, walks the AST of the resulting wire_gen.go, computes
// lifecycle ordering, and emits lifecycle_gen.go into the target
// application package.
//
// See docs/adr/ADR-002-wire-as-graph-source.md and
// docs/adr/ADR-008-parse-generated-wire-output.md. This package must import
// only Google Wire's public types, never Google Wire's unexported
// implementation packages.
//
// A Generator, built from Options with NewGenerator, runs Google Wire over
// one or more packages. It parses each injector into a graph of creation
// events, dependency edges, results, and cleanup functions. Wire's
// wire_gen.go is transient. A run leaves the directory as it found it: the
// run preserves a wire_gen.go that it did not create, and removes the one
// that it did.
//
// Analyze turns a parsed package into one Analysis per injector: each
// component's lifecycle capabilities, which static type analysis of the
// package resolves, and the single dependency-ordered level list that Yama
// computes over them.
//
// The parser is coupled to the shape of Google Wire's generated output. The
// parser is validated against Google Wire v0.7.0, which go.mod pins. Google
// Wire is archived, so that shape does not move.
package generator
