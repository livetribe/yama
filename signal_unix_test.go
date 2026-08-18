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

//go:build unix

package yama_test

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/mocks"
)

// These tests deliver signals to the test process itself. An independent,
// always-installed registration keeps the process from ever reverting to the
// default (terminating) disposition for those signals, so a self-delivery cannot
// race the handler and kill the run. Every signal any test below raises must be
// in this set.
func TestMain(m *testing.M) {
	safety := make(chan os.Signal, 16)
	signal.Notify(safety, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		for range safety {
		}
	}()

	os.Exit(m.Run())
}

// raise delivers sig to the current process.
func raise(t *testing.T, sig syscall.Signal) {
	t.Helper()
	require.NoError(t, syscall.Kill(syscall.Getpid(), sig))
}

// launch runs RunUntilSignal in a goroutine, reporting its result over the
// returned channel.
func launch(lc *mocks.MockLifecycle, watch ...os.Signal) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- yama.RunUntilSignal(context.Background(), lc, watch...)
	}()

	return done
}

// awaitReturn fails the test unless RunUntilSignal returns within the window, and
// yields its error otherwise.
func awaitReturn(t *testing.T, done <-chan error, msg string) error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
		return nil
	}
}

// startNotifier returns a Start expectation that reports, over the returned
// channel, that RunUntilSignal has entered Start — hence installed its handler.
func startNotifier(lc *mocks.MockLifecycle) <-chan struct{} {
	started := make(chan struct{})
	lc.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(started)
		return nil
	})

	return started
}

// runHappyPath drives the successful shape once: start, deliver a signal after
// the handler is installed, then Stop and a nil return. watch is passed through
// to RunUntilSignal, so a nil watch exercises the empty-signals default set.
func runHappyPath(t *testing.T, watch []os.Signal, deliver syscall.Signal) {
	t.Helper()

	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := startNotifier(lc)
	lc.EXPECT().Stop(gomock.Any()).Times(1)

	done := launch(lc, watch...)

	<-started
	raise(t, deliver)

	assert.NoError(t, awaitReturn(t, done, "RunUntilSignal did not return after the signal was delivered"))
}

// TestRunUntilSignalStopsOnSignal is the happy path: a caller-named signal drives
// a successful start through to Stop.
func TestRunUntilSignalStopsOnSignal(t *testing.T) {
	runHappyPath(t, []os.Signal{syscall.SIGTERM}, syscall.SIGTERM)
}

// TestRunUntilSignalDefaultInterrupt proves the empty-signals default set honors
// the interrupt signal.
func TestRunUntilSignalDefaultInterrupt(t *testing.T) {
	runHappyPath(t, nil, syscall.SIGINT)
}

// TestRunUntilSignalDefaultTermination proves the empty-signals default set also
// honors the termination signal, the other half of the PRD default.
func TestRunUntilSignalDefaultTermination(t *testing.T) {
	runHappyPath(t, nil, syscall.SIGTERM)
}

// TestRunUntilSignalCustomSignal proves a caller-named signal outside the default
// set drives shutdown.
func TestRunUntilSignalCustomSignal(t *testing.T) {
	runHappyPath(t, []os.Signal{syscall.SIGUSR1}, syscall.SIGUSR1)
}

// TestRunUntilSignalIgnoresUnwatchedSignal proves the watch set is selective: a
// signal outside it does not end the wait, while the watched signal still does.
// The watched delivery at the end also unblocks the goroutine, so nothing leaks
// into later tests that share these process-wide signals.
func TestRunUntilSignalIgnoresUnwatchedSignal(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := startNotifier(lc)
	lc.EXPECT().Stop(gomock.Any()).Times(1)

	done := launch(lc, syscall.SIGUSR1)

	<-started
	raise(t, syscall.SIGTERM)

	select {
	case <-done:
		t.Fatal("RunUntilSignal returned on a signal outside its watch set")
	case <-time.After(200 * time.Millisecond):
	}

	raise(t, syscall.SIGUSR1)
	assert.NoError(t, awaitReturn(t, done, "RunUntilSignal did not return on its watched signal"))
}

// TestRunUntilSignalHonorsSignalRaisedDuringStart proves the handler is installed
// before Start and the channel is buffered: a signal delivered while Start is
// still running drives shutdown once Start returns.
func TestRunUntilSignalHonorsSignalRaisedDuringStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	inStart := make(chan struct{})
	releaseStart := make(chan struct{})
	lc.EXPECT().Start(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(inStart)
		<-releaseStart
		return nil
	})
	lc.EXPECT().Stop(gomock.Any()).Times(1)

	done := launch(lc, syscall.SIGTERM)

	<-inStart
	raise(t, syscall.SIGTERM)
	// Let the runtime forward the signal while Start is still parked.
	time.Sleep(100 * time.Millisecond)
	close(releaseStart)

	assert.NoError(t, awaitReturn(t, done, "RunUntilSignal dropped a signal raised during startup"))
}

// TestRunUntilSignalSecondSignalDoesNotReenterStop delivers a second watched
// signal while Stop is still running. Stop must run exactly once — the Times(1)
// expectation fails the test if it re-enters.
func TestRunUntilSignalSecondSignalDoesNotReenterStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	lc := mocks.NewMockLifecycle(ctrl)

	started := startNotifier(lc)

	stopEntered := make(chan struct{})
	releaseStop := make(chan struct{})
	lc.EXPECT().Stop(gomock.Any()).Times(1).Do(func(context.Context) {
		close(stopEntered)
		<-releaseStop
	})

	done := launch(lc, syscall.SIGTERM)

	<-started
	raise(t, syscall.SIGTERM)

	<-stopEntered
	raise(t, syscall.SIGTERM)
	close(releaseStop)

	assert.NoError(t, awaitReturn(t, done, "RunUntilSignal did not return after shutdown completed"))
}
