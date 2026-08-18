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

// Package work runs the three phases of a run over one target package.
//
// A package enters the run as an item, and the item holds a State. The run
// calls Prepare on every item. Then the run calls Google Wire one time. Then
// the run calls Generate on every item. Last, the run calls Complete on every
// item.
//
// Each phase returns a State. A phase that succeeds returns the same value. A
// phase that fails returns a value of a different type. That type generates
// nothing more for that package. The type of the item is therefore the record
// of what happened to that package. An item holds no phase field and no error
// flag.
//
// Complete is the one call that every state answers. Its return value is the
// package's error. A state that the run cannot reach in a phase panics in that
// phase's method.
package work
