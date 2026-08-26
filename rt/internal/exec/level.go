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

package exec

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"l7e.io/yama/v2"
)

// Level is one set of members with no ordering between them. Every pass runs all
// of them concurrently and waits for all of them, so a Level behaves as a single
// component to the lifecycle driving it.
type Level []CompleteLifecycle

var _ CompleteLifecycle = Level(nil)

// Start runs every member and waits for all of them to settle. It reports
// ErrStartFailed if any member returned an error or panicked. A member's failure
// does not cancel its siblings: each runs to its own conclusion, leaving every
// member either started or failed on its own terms.
func (l Level) Start(ctx context.Context) error {
	err := new(atomic.Bool)

	l.spawn(ctx, func(ctx context.Context, c CompleteLifecycle) {
		defer func() {
			if r := recover(); r != nil {
				// Record the failure, then re-panic so spawn contains and
				// reports it. Swallowing it here would lose the report.
				err.Store(true)
				panic(r)
			}
		}()

		if e := c.Start(ctx); e != nil {
			err.Store(true)
		}
	})

	if err.Load() {
		return yama.ErrStartFailed
	}

	return nil
}

// Quiesce runs every member and waits for all of them. It reports nothing. A
// member that panics does not stop the rest.
func (l Level) Quiesce(ctx context.Context) {
	l.spawn(ctx, func(ctx context.Context, c CompleteLifecycle) {
		c.Quiesce(ctx)
	})
}

// Release runs every member's Release and waits for all of them. It reports
// nothing. A member that panics does not stop the rest.
func (l Level) Release(ctx context.Context) {
	l.spawn(ctx, func(ctx context.Context, c CompleteLifecycle) {
		c.Release(ctx)
	})
}

// Stop runs every member and waits for all of them. It reports nothing. A
// member that panics does not stop the rest.
func (l Level) Stop(ctx context.Context) {
	l.spawn(ctx, func(ctx context.Context, c CompleteLifecycle) {
		c.Stop(ctx)
	})
}

// spawn runs fn against each member in its own goroutine and waits for all of
// them. A panic is recovered and logged; it never escapes to the caller, and it
// never prevents the other members from running to completion.
func (l Level) spawn(ctx context.Context, fn func(ctx context.Context, c CompleteLifecycle)) {
	var wg sync.WaitGroup

	for _, c := range l {
		wg.Add(1)
		go func(cc CompleteLifecycle) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())

					slog.ErrorContext(ctx, "panic recovered",
						slog.Any("error", r),
						slog.String("stack", stack),
					)
				}
			}()

			fn(ctx, cc)
		}(c)
	}

	wg.Wait()
}
