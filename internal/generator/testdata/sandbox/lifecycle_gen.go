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

//go:build !wireinject && !yamainject

package sandbox

// Hand-authored stand-in for a lifecycle_gen.go, present so a load of this
// fixture has two generated files to choose between. Nothing parses it.
//
// The levels below are the ones this graph yields:
//
//	level 0: base1, base2 (cleanup only), base3
//	level 1: mid2, root2
//	level 2: root3

import (
	"io"
	"os"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/rt"
)

// NewLifecycle re-emits InitializeApp's construction, wraps every lifecycle
// component through the interceptor chains, and returns the application beside
// its Lifecycle. On construction failure it returns nil, nil, err. On success
// every Wire cleanup runs as part of Lifecycle.Stop and nowhere else, added with
// WithCleanup when its value implements no capability and WithCleanableComponent
// when it does.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	// --- re-emitted from wire_gen.go: InitializeApp, minus its final return ---
	// Value/InterfaceValue providers are reproduced as their own expressions, not
	// by referencing Wire's private _wire*Value vars.
	writer := os.Stdout
	config := Config{Env: Prod, LogPrefix: "[sandbox] "}
	consoleLogger := NewConsoleLogger(writer, config)
	base1 := NewBase1(consoleLogger)
	base2, base2Cleanup, err := NewBase2(consoleLogger)
	if err != nil {
		return nil, nil, err
	}
	mid1 := NewMid1(consoleLogger, base1, base2)
	root1 := NewRoot1(consoleLogger, mid1)
	root2 := NewRoot2(consoleLogger, mid1)
	environment := config.Env
	base3, base3Cleanup := NewBase3(consoleLogger, environment)
	mid2 := NewMid2(consoleLogger, base2, base3)
	root3 := NewRoot3(consoleLogger, mid2)
	app := &App{
		root1: root1,
		root2: root2,
		root3: root3,
	}
	// --- end re-emitted construction ---

	b := rt.NewLifecycleBuilder(opts...)
	b.NextLevel().
		WithComponents(base1).
		WithCleanup(base2Cleanup).
		WithCleanableComponent(base3, base3Cleanup).
		Add()
	b.NextLevel().
		WithComponents(mid2).
		WithComponents(root2).
		Add()
	b.NextLevel().
		WithComponents(root3).
		Add()
	lc := b.Build()

	return app, lc, nil
}

// NewLifecycleWithWriter is the constructor for the InitializeAppWithWriter
// injector. There is one constructor per stub in lifecycle.go, each carrying its
// own injector's parameters ahead of opts: w is threaded through the re-emitted
// body in place of the os.Stdout interface value. Each constructor declares its
// own levels inline and shares nothing with the others, whether or not their
// graphs match.
func NewLifecycleWithWriter(w io.Writer, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	// --- re-emitted from wire_gen.go: InitializeAppWithWriter, minus its return ---
	config := Config{Env: Prod, LogPrefix: "[sandbox] "}
	consoleLogger := NewConsoleLogger(w, config)
	base1 := NewBase1(consoleLogger)
	base2, base2Cleanup, err := NewBase2(consoleLogger)
	if err != nil {
		return nil, nil, err
	}
	mid1 := NewMid1(consoleLogger, base1, base2)
	root1 := NewRoot1(consoleLogger, mid1)
	root2 := NewRoot2(consoleLogger, mid1)
	environment := config.Env
	base3, base3Cleanup := NewBase3(consoleLogger, environment)
	mid2 := NewMid2(consoleLogger, base2, base3)
	root3 := NewRoot3(consoleLogger, mid2)
	app := &App{
		root1: root1,
		root2: root2,
		root3: root3,
	}
	// --- end re-emitted construction ---

	b := rt.NewLifecycleBuilder(opts...)
	b.NextLevel().
		WithComponents(base1).
		WithCleanup(base2Cleanup).
		WithCleanableComponent(base3, base3Cleanup).
		Add()
	b.NextLevel().
		WithComponents(mid2).
		WithComponents(root2).
		Add()
	b.NextLevel().
		WithComponents(root3).
		Add()
	lc := b.Build()

	return app, lc, nil
}
