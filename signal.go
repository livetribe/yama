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

package yama

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// RunUntilSignal is the typical main entry point for a Yama application: it
// starts lc, blocks until one of the given OS signals is delivered, then calls
// lc.Stop and returns. When signals is empty it waits on the interrupt and
// termination signals. If Start fails it returns an error matching ErrStartFailed
// without waiting for a signal.
//
// A signal arriving during startup is honored once startup completes; a further
// signal arriving during shutdown is ignored.
func RunUntilSignal(lc Lifecycle, signals ...os.Signal) error {
	if len(signals) == 0 {
		signals = defaultSignals
	}

	// signal.Notify drops a delivery when the channel is not ready, so the buffer
	// must hold the one that can arrive before the receive below.
	received := make(chan os.Signal, 1)
	signal.Notify(received, signals...)
	defer signal.Stop(received)

	if err := lc.Start(context.Background()); err != nil {
		return err
	}

	<-received

	lc.Stop(context.Background())

	return nil
}

// defaultSignals is the set RunUntilSignal waits on when the caller names none:
// the interrupt and termination signals.
var defaultSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
