// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
