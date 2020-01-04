// +build linux bsd darwin

/*
 * Copyright (c) 2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package yama_test

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"l7e.io/yama"
)

func Example_wait() {
	server := &http.Server{Addr: ":0", Handler: nil}

	var flushed sync.WaitGroup

	flushed.Add(1)

	watcher := yama.NewWatcher(
		yama.WatchingSignals(syscall.SIGINT, syscall.SIGTERM),
		yama.WithTimeout(2*time.Second),
		yama.WithClosers(
			server,
			yama.FnAsCloser(func() { fmt.Println("Signal caught"); _ = os.Stdout.Sync(); flushed.Done() })))

	// simulate a later signal
	go func() {
		time.Sleep(100 * time.Millisecond)

		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("error serving traffic %s\n", err)
	}

	_ = watcher.Wait()

	flushed.Wait() // wait until stdout is flushed

	// Output:
	// Signal caught
}
