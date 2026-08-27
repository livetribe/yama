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

// This file holds the lifecycle stub that a person wrote.

//go:build yamainject

package app

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama"
)

// NewAppLifecycle builds the app and its lifecycle.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewDB, NewApp))
}
