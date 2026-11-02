// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Command hello runs the example application until it is interrupted.
package main

import (
	"context"
	"log"
	"os"

	"l7e.io/yama"

	"example.com/hello"
)

func main() {
	app, lc, err := hello.NewLifecycle(os.Stdout)
	if err != nil {
		log.Fatalf("hello: construction failed: %v", err)
	}

	log.Printf("hello: built %s", app)

	if err := yama.RunUntilSignal(context.Background(), lc); err != nil {
		log.Fatalf("hello: %v", err)
	}
}
