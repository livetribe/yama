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

// Command hello runs the example application until it is interrupted.
package main

import (
	"context"
	"log"
	"os"

	yama "l7e.io/yama"

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
