// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package yama

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// RunUntilSignal is the typical main entry point for a Yama application. It
// starts lc, then blocks until one of the given OS signals arrives, or until ctx
// is done. It then calls lc.Stop and returns. When signals is empty,
// RunUntilSignal waits on the interrupt and termination signals. If Start fails,
// RunUntilSignal returns an error that matches ErrStartFailed.
//
// RunUntilSignal gives ctx to lc.Start and to lc.Stop without a change. The
// values, a cancellation, and a deadline on ctx therefore reach each component
// and each interceptor. Each component decides how to respond. A ctx that is
// done is not a failure, so RunUntilSignal returns nil.
//
// If a signal arrives during startup, RunUntilSignal honors it once startup
// completes. RunUntilSignal ignores a further signal that arrives during shutdown.
func RunUntilSignal(ctx context.Context, lc Lifecycle, signals ...os.Signal) error {
	if len(signals) == 0 {
		signals = defaultSignals
	}

	waitCtx, stopNotify := signal.NotifyContext(ctx, signals...)
	defer stopNotify()

	if err := lc.Start(ctx); err != nil {
		return err
	}

	<-waitCtx.Done()

	lc.Stop(ctx)

	return nil
}

// defaultSignals is the set RunUntilSignal waits on when the caller names none:
// the interrupt and termination signals.
var defaultSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
