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

// Package testapp is a generation target whose committed lifecycle_gen.go the
// behavioral contract runs against. Its components record every lifecycle call
// they receive, so a spec can assert the ordering, the failure handling, and the
// teardown that the generated constructor and the runtime-support package
// produce together.
//
// Yama emits lifecycle_gen.go here. Edit the stub in lifecycle.go and
// regenerate; do not edit the generated file.
package testapp
