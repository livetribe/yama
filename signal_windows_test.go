// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package yama_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"l7e.io/yama"
	"l7e.io/yama/internal/mocks"
)

// TestRunUntilSignalBlocksAfterStart is the Windows-tagged counterpart to the
// unix delivery tests: Windows offers no portable way to self-deliver a console
// signal in a unit test, so rather than driving Stop, this proves the same code
// compiles under the Windows build, starts lc, and reaches the signal wait
// without returning or calling Stop first. The goroutine stays parked on that
// wait; the test process reclaims it on exit.
func TestRunUntilSignalBlocksAfterStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := make(chan struct{})
	lc.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(started)
		return nil
	})
	// No Stop expectation: with no signal delivered, Stop must not run.

	done := make(chan error, 1)
	go func() {
		done <- yama.RunUntilSignal(context.Background(), lc, os.Interrupt)
	}()

	<-started

	select {
	case <-done:
		t.Fatal("RunUntilSignal returned before any signal was delivered")
	case <-time.After(100 * time.Millisecond):
	}
}
