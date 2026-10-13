// Copyright the original author or authors.
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

// This file binds the name that the stub file binds to another package. That
// package declares a Context type. The output does not import the stub file's
// package. Generate gives that package a free name in the lifecycle file.

//go:build !wireinject

package app

import context "go/build"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	logger := NewLogger()
	db, cleanup := NewDB(logger)
	app := NewApp(db)
	return app, func() { cleanup() }, nil
}
