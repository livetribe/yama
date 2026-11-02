// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
