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

// Package sandbox is the frozen fixture the parser and analysis tests read: a
// component graph wired end to end, with Wire's wire_gen.go and a
// lifecycle_gen.go checked in beside it.
//
// wire_gen.go is hand-massaged. Tests load this package rather than running
// Wire over it, so regenerating wire_gen.go discards the massaging the parser
// oracle asserts against. lifecycle_gen.go is present so a load has two
// generated files to choose between.
//
// The fixture exercises the breadth of Wire's feature set:
//
//   - injectors (InitializeApp, InitializeAppWithWriter) and injector arguments
//   - provider functions and composed provider sets (wire.NewSet)
//   - interface binding (wire.Bind: Logger to *ConsoleLogger)
//   - struct assembly (wire.Struct: App from its fields)
//   - value providers (wire.Value: Config) and interface values
//     (wire.InterfaceValue: os.Stdout as io.Writer)
//   - field projection (wire.FieldsOf: Config.Env)
//   - cleanup and error propagation (NewBase2 returns func() and error)
//
// Dependency edges (parent depends on child):
//
//	root1 → mid1
//	root2 → mid1
//	root3 → mid2
//	mid1  → base1, base2
//	mid2  → base2, base3
//
// Each component optionally implements the yama lifecycle interfaces (Starter,
// Quiescer, Stopper); the compile-time assertions in components.go record which.
package sandbox
