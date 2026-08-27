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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	yama "l7e.io/yama"
	"l7e.io/yama/internal/mocks"
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
		done <- yama.RunUntilSignal(context.Background(), lc)
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, yama.ErrStartFailed)
	case <-time.After(2 * time.Second):
		t.Fatal("RunUntilSignal blocked after Start failed instead of returning")
	}
}

// ctxKey identifies the value that the context tests below attach.
type ctxKey struct{}

// TestRunUntilSignalStopsOnContextCancellation proves that a cancellation of the
// caller's context ends the wait in place of a signal. It also proves that the
// context that Stop receives carries that cancellation. This test delivers no
// signal, so it runs on every platform.
func TestRunUntilSignalStopsOnContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := make(chan struct{})
	lc.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(started)
		return nil
	})

	stopErr := make(chan error, 1)
	lc.EXPECT().Stop(gomock.Any()).Times(1).Do(func(ctx context.Context) {
		stopErr <- ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- yama.RunUntilSignal(ctx, lc)
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("RunUntilSignal did not return after the context was canceled")
	}

	assert.ErrorIs(t, <-stopErr, context.Canceled)
}

// TestRunUntilSignalPassesContextValuesToStartAndStop proves that the caller's
// context reaches both lifecycle calls. Start and Stop both read the value that
// the context carries.
func TestRunUntilSignalPassesContextValuesToStartAndStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := make(chan struct{})
	startValue := make(chan any, 1)
	lc.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		startValue <- ctx.Value(ctxKey{})
		close(started)
		return nil
	})

	stopValue := make(chan any, 1)
	lc.EXPECT().Stop(gomock.Any()).Times(1).Do(func(ctx context.Context) {
		stopValue <- ctx.Value(ctxKey{})
	})

	valued := context.WithValue(context.Background(), ctxKey{}, "node-7")
	ctx, cancel := context.WithCancel(valued)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- yama.RunUntilSignal(ctx, lc)
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("RunUntilSignal did not return after the context was canceled")
	}

	assert.Equal(t, "node-7", <-startValue)
	assert.Equal(t, "node-7", <-stopValue)
}
