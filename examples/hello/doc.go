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

// Package hello is a small Yama application, in its own Go module.
//
// components.go declares the graph. lifecycle.go declares one stub for the
// constructor the application calls. lifecycle_gen.go is what Yama emits from
// them, and is the only generated file this package commits.
//
// The graph is three components:
//
//	server → store → config
//
// Config carries no lifecycle capability, so it is a dependency alone. Store
// starts and stops, and its provider returns a cleanup function. Server starts,
// quiesces, and stops.
//
// The directive below regenerates lifecycle_gen.go. Run it with
// `go generate ./...`.
//
//go:generate go tool yama
package hello
