// +build linux bsd darwin

package yama_test

/*
 * Copyright (c) 2019 the original author or authors.
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

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"l7e.io/yama"
)

type Unhashable map[string]interface{}

var uCalled int

func (u Unhashable) Close() error {
	uCalled++
	return nil
}

type neverClose struct {
	wg     sync.WaitGroup
	Closed int
}

func (n *neverClose) Close() error {
	n.Closed++
	n.wg.Done()
	time.Sleep(2 * time.Minute)
	return nil
}

// CloseMe returns nil from close
type CloseMe struct {
	Closed int
}

func (c *CloseMe) Close() error {
	c.Closed++
	return nil
}

// BadCloser returns an error from close
type BadCloser struct {
	Closed int
}

func (c *BadCloser) Close() error {
	c.Closed++
	return fmt.Errorf("error from bad closer")
}

func TestYama(t *testing.T) {

	Convey("Validate watcher handles unhashable types", t, func() {
		u := make(Unhashable)
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGTERM),
			yama.WithClosers(u))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(uCalled, ShouldEqual, 1)
	})

	Convey("Validate watcher observes cleanly", t, func() {
		watcher := yama.NewWatcher(yama.WatchingSignals(syscall.SIGTERM))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
	})

	Convey("Validate watcher not affected by bad closers", t, func() {
		bad := &BadCloser{}
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGTERM),
			yama.WithClosers(bad))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(bad.Closed, ShouldEqual, 1)
	})

	Convey("Validate watcher observes other signals", t, func() {
		closeMe := &CloseMe{}
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(closeMe))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)
	})

	Convey("Validate watcher notifies closers when closed", t, func() {
		closeMe := &CloseMe{}
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(closeMe))

		err := watcher.Close()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)
	})

	Convey("Validate watcher when closed twice", t, func() {
		closeMe := &CloseMe{}
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(closeMe))

		err := watcher.Close()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)

		// must not block and closers should only be called once
		err = watcher.Close()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)
	})

	Convey("Validate watcher when signaled several times", t, func() {
		closeMe := &CloseMe{}
		watcher := yama.NewWatcher(
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(closeMe))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		}()

		err := watcher.Wait()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)

		_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		err = watcher.Wait()
		So(err, ShouldBeNil)
		So(closeMe.Closed, ShouldEqual, 1)
	})

	Convey("Validate watcher gives up after notification timeout", t, func() {
		neverClose := &neverClose{}
		neverClose.wg.Add(1)

		watcher := yama.NewWatcher(
			yama.WithTimeout(10*time.Millisecond),
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(neverClose))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		}()

		err := watcher.Wait()
		So(err, ShouldNotBeNil)
		So(err, ShouldHaveSameTypeAs, &yama.ErrTimedOut{})
		So(err, ShouldImplement, (*error)(nil))
		So(err.(*yama.ErrTimedOut).Uncompleted, ShouldResemble, []io.Closer{neverClose})

		neverClose.wg.Wait()
		So(neverClose.Closed, ShouldEqual, 1)

		// Second wait should return the same error.
		err = watcher.Wait()
		So(err, ShouldNotBeNil)
		So(err, ShouldHaveSameTypeAs, &yama.ErrTimedOut{})
		So(err, ShouldImplement, (*error)(nil))
		So(err.(*yama.ErrTimedOut).Uncompleted, ShouldResemble, []io.Closer{neverClose})
	})

	Convey("Notify multiple closers with one closer that fails the timer", t, func() {
		neverClose := &neverClose{}
		neverClose.wg.Add(1)
		closeMe := &CloseMe{}
		watcher := yama.NewWatcher(
			yama.WithTimeout(10*time.Millisecond),
			yama.WatchingSignals(syscall.SIGHUP),
			yama.WithClosers(neverClose, closeMe))
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		}()

		err := watcher.Wait()
		So(err, ShouldNotBeNil)
		So(err, ShouldHaveSameTypeAs, &yama.ErrTimedOut{})
		So(err, ShouldImplement, (*error)(nil))
		So(err.(*yama.ErrTimedOut).Uncompleted, ShouldResemble, []io.Closer{neverClose})

		// ensure all closers called
		neverClose.wg.Wait()
		So(neverClose.Closed, ShouldEqual, 1)
		So(closeMe.Closed, ShouldEqual, 1)
	})
}
