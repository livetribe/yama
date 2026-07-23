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

package main

import (
	"context"
	"fmt"
	"log"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/generator/sandbox"
)

// observer is a global interceptor that prints each component's operation as it
// runs, so the start/quiesce/stop order is visible. It implements all three
// interceptor interfaces, so it joins every chain.
type observer struct{}

func (observer) Start(ctx context.Context, next yama.Starter) error {
	fmt.Printf("  start   %s\n", identify(ctx))
	return next.Start(ctx)
}

func (observer) Quiesce(ctx context.Context, next yama.Quiescer) {
	fmt.Printf("  quiesce %s\n", identify(ctx))
	next.Quiesce(ctx)
}

func (observer) Stop(ctx context.Context, next yama.Stopper) {
	fmt.Printf("  stop    %s\n", identify(ctx))
	next.Stop(ctx)
}

func identify(ctx context.Context) string {
	c, _ := yama.FromContext[any](ctx)
	if s, ok := c.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%T", c)
}

func main() {
	ctx := context.Background()

	// Boundary components carry no Wire providers; they are supplied here, as
	// runtime objects, and never enter the graph.
	fmt.Println("== construct via NewLifecycle (re-emitted Wire body) ==")
	app, lc, err := sandbox.NewLifecycle(
		yama.WithInterceptors(observer{}),
		yama.WithBeginComponents(&sandbox.B1{}, &sandbox.B2{}, &sandbox.B3{}),
		yama.WithEndComponents(&sandbox.E1{}, &sandbox.E2{}),
	)
	if err != nil {
		log.Fatalf("construction failed: %v", err)
	}

	fmt.Println("\n== wired dependency tree ==")
	fmt.Println(app)

	// Cross-level order is deterministic; order within a level is not, since a
	// level's members run concurrently.
	fmt.Println("\n== Start (begin boundary, then graph L0→L2, then end boundary) ==")
	if err := lc.Start(ctx); err != nil {
		log.Fatalf("start failed: %v", err)
	}

	fmt.Println("\n== Stop (quiesce pass, then teardown; end boundary first, begin last) ==")
	lc.Stop(ctx)
}
