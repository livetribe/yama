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

//go:build windows

package yama_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/mocks"
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
		done <- yama.RunUntilSignal(lc, os.Interrupt)
	}()

	<-started

	select {
	case <-done:
		t.Fatal("RunUntilSignal returned before any signal was delivered")
	case <-time.After(100 * time.Millisecond):
	}
}
