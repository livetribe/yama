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

package yama_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/mocks"
)

// TestRunUntilSignalStartFailureShortCircuits proves the Start-failure path never
// reaches the signal wait: it returns an error matching ErrStartFailed, does not
// call Stop, and does not block. No signal is delivered, so a regression that
// waited would hang — the timeout turns that into a clear failure.
func TestRunUntilSignalStartFailureShortCircuits(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)
	// No Stop expectation: a Stop on start failure would fail the test.
	lc.EXPECT().Start(gomock.Any()).Return(yama.ErrStartFailed)

	done := make(chan error, 1)
	go func() {
		done <- yama.RunUntilSignal(lc)
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, yama.ErrStartFailed)
	case <-time.After(2 * time.Second):
		t.Fatal("RunUntilSignal blocked after Start failed instead of returning")
	}
}
