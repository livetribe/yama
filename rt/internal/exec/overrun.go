// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"l7e.io/yama"
	"l7e.io/yama/internal/bridge"
)

// overrunMessage is the log message emitted when a component's operation returns
// after the caller's context deadline.
const overrunMessage = "lifecycle component exceeded its deadline"

// overrun is the built-in per-component overrun interceptor. It is stateless and
// shared across every component and operation. It waits for next to return, then
// logs once, at Warn level via the log/slog default logger, when the operation
// returned after the caller's context deadline. It never cancels or abandons the
// operation.
var overrun overrunInterceptor

// Every component's chain carries overrun in all three operations.
var (
	_ yama.StartInterceptor   = overrunInterceptor{}
	_ yama.QuiesceInterceptor = overrunInterceptor{}
	_ yama.StopInterceptor    = overrunInterceptor{}
)

type overrunInterceptor struct{}

func (overrunInterceptor) Start(ctx context.Context, next yama.Starter) error {
	err := next.Start(ctx)
	logOverrun(ctx)
	return err
}

func (overrunInterceptor) Quiesce(ctx context.Context, next yama.Quiescer) {
	next.Quiesce(ctx)
	logOverrun(ctx)
}

func (overrunInterceptor) Stop(ctx context.Context, next yama.Stopper) {
	next.Stop(ctx)
	logOverrun(ctx)
}

// logOverrun emits one Warn record when ctx carried a deadline that had already
// passed by the time the operation returned. The record attributes the overrun to
// the component and reports how far past the deadline the operation ran.
func logOverrun(ctx context.Context) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return
	}

	now := time.Now()
	if !now.After(deadline) {
		return
	}

	slog.Default().LogAttrs(ctx, slog.LevelWarn, overrunMessage,
		slog.String("component", componentIdentity(ctx)),
		slog.Duration("overrun", now.Sub(deadline)),
	)
}

// componentIdentity renders the component attached to ctx: its String when it is a
// fmt.Stringer, otherwise its %T type.
func componentIdentity(ctx context.Context) string {
	c, ok := bridge.FromContext[any](ctx)
	if !ok {
		return "unknown"
	}

	if s, ok := c.(fmt.Stringer); ok {
		return s.String()
	}

	return fmt.Sprintf("%T", c)
}
