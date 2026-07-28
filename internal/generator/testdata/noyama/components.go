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

// Package noyama declares a lifecycle-capable component without importing yama.
// A component implements a capability by declaring the method, and nothing about
// this package refers to yama.
package noyama

import "context"

// Worker implements Starter and Stopper.
type Worker struct{}

func NewWorker() *Worker {
	return &Worker{}
}

func (*Worker) Start(context.Context) error { return nil }
func (*Worker) Stop(context.Context)        {}

// App is the graph's root and implements no lifecycle interface.
type App struct {
	worker *Worker
}
