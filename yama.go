/*
 * Copyright (c) 2019-20 the original author or authors.
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

/*
Package yama provides a signal watcher that can be used to shutdown an application.

A signal watcher can be constructed to watch any number of signals and will
call any number of registered io.Closer instances, when such signals occur; the
results of calling Close() on the registered instances are ignored.

	watcher := yama.NewWatcher(
		yama.WatchingSignals(syscall.SIGINT, syscall.SIGTERM),
		yama.WithTimeout(2*time.Second),
		yama.WithClosers(server))

An application can wait fir the completion of the Closer notifications by
calling the blocking method, Wait().

    watcher.Wait()

Here, the caller will be blocked until one of the signals occur and all the
Closer notifications have either completed or two seconds have elapsed since
the start of Closer notifications; the timeout is set above by passing
yama.WithTimeout().  Subsequent signals will not trigger Closer notifications.

The application can programmatically trigger Closer notifications by calling

    watcher.Close()

If this is done, subsequent signals will not trigger Closer notifications.

There are a few helper methods, FnAsCloser() and ErrValFnAsCloser(), that can
be used to wrap simple functions and functions that can return an error,
respectively, into instances that implement io.Closer.
*/
package yama // import "l7e.io/yama"

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"
)

// DefaultTimeout is the default closer timeout of watcher instances.
const DefaultTimeout = 10 * time.Second

// ErrTimedOut is an error that contains the set of closers that didn't complete
// before the configured timeout.
type ErrTimedOut struct {
	Uncompleted []io.Closer
}

func (e *ErrTimedOut) Error() string {
	return "closers timed out"
}

// Watcher notifies configured closers when a configured signal occurred or
// when the instance is closed.  Closers are only called once.
//
// See the package documentation for details.
type Watcher struct {
	wg      sync.WaitGroup
	signals chan os.Signal
	done    chan struct{}
	timeout time.Duration
	closers []io.Closer
	once    sync.Once
	err     error
}

// holder is a wrapper to the struct we are going to close with metadata
// to help with debugging close.
type holder struct {
	key    int
	closer io.Closer
}

// NewWatcher creates Watcher with various options.
func NewWatcher(options ...Option) (yama *Watcher, err error) {
	w := &Watcher{
		signals: make(chan os.Signal, 1),
		done:    make(chan struct{}, 1),
	}

	s := &Settings{TimeOut: DefaultTimeout}

	for _, option := range options {
		option.Apply(s)
	}

	for i, closer := range s.Closers {
		if closer == nil {
			return nil, fmt.Errorf("closer #%d must not be null", i)
		}
	}

	w.timeout = s.TimeOut
	w.closers = s.Closers

	signal.Notify(w.signals, s.Signals...)

	// The wait group will be marked done when a signal is observed or the
	// watcher receives done.
	w.wg.Add(1)

	go func() {
		defer func() {
			w.notify()
			w.wg.Done()
		}()

		for {
			select {
			case <-w.signals:
				return
			case <-w.done:
				return
			}
		}
	}()

	return w, nil
}

// Wait until the configured signal occurs or the instance is closed.
func (w *Watcher) Wait() error {
	w.wg.Wait()

	return w.err
}

// Close the instance, notifying any registered closers. Can be called
// multiple times, but closers will only be called once.
func (w *Watcher) Close() error {
	w.done <- struct{}{}
	w.notify()

	return w.err
}

// Notify closers, ensuring they are only called once.
func (w *Watcher) notify() {
	w.once.Do(w.notifyClosers)
}

// notifyClosers calls all closers once and wait for them to finish with a
// channel.  If not all closers return within the timeout, returns an error
// with the tardy closers.
func (w *Watcher) notifyClosers() {
	count := len(w.closers)
	if count == 0 {
		return
	}

	pending := make(map[int]holder)
	completed := make(chan holder, count)

	for i, closer := range w.closers {
		h := holder{key: i, closer: closer}

		go func() {
			_ = h.closer.Close()
			completed <- h
		}()

		pending[i] = h
	}

	// wait on channels for notifications
	for {
		select {
		case <-time.After(w.timeout):
			var uncompleted []io.Closer
			for _, h := range pending {
				uncompleted = append(uncompleted, h.closer)
			}

			w.err = &ErrTimedOut{Uncompleted: uncompleted}

			return
		case closer := <-completed:
			delete(pending, closer.key)
			count--

			if count == 0 || len(pending) == 0 {
				return
			}
		}
	}
}

// FnAsCloser wraps a function in a Closer instance, called when the instance's
// Close() method is called; the method always returns nil.
func FnAsCloser(f func()) io.Closer {
	return &fnWrapper{f: f}
}

type fnWrapper struct {
	f func()
}

func (w *fnWrapper) Close() error {
	w.f()
	return nil
}

// ErrValFnAsCloser wraps a function which can return an error in a Closer
// instance, called when the instance's Close() method is called; the method's
// value is the value returned by the function.
func ErrValFnAsCloser(f func() error) io.Closer {
	return &errValFnWrapper{f: f}
}

type errValFnWrapper struct {
	f func() error
}

func (w *errValFnWrapper) Close() error {
	return w.f()
}
