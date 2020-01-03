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
	"os"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"l7e.io/yama"
)

func TestHelpers(t *testing.T) {

	Convey("Ensure wrapped simple functions are called", t, func() {
		called := false
		c := yama.FnAsCloser(func() {
			called = true
		})

		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGTERM),
			yama.WithClosers(c))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)
	})

	Convey("Ensure wrapped functions that can return errors are called", t, func() {
		called := false
		c := yama.ErrValFnAsCloser(func() error {
			called = true
			return nil
		})

		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGTERM),
			yama.WithClosers(c))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)
	})
}
