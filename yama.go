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

package yama // import "l7e.io/yama"

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"time"
)

// DefaultTimeout is the default closer timeout of watcher instances.
const DefaultTimeout = 10 * time.Second

// TimedOut is an error that contains the set of closers that didn't complete
// before the configured timeout.
type TimedOut struct {
	Uncompleted []io.Closer
}

func (t *TimedOut) Error() string {
	return "closers timed out"
}

// Watcher notifies configured closers when a configured signal occurred or
// when the instance is closed.  Closers are only called once.
type Watcher struct {
	wg      sync.WaitGroup
	signals chan os.Signal
	done    chan struct{}
	timeout time.Duration
	closers []io.Closer
	mux     sync.Mutex
}

// holder is a wrapper to the struct we are going to close with metadata
// to help with debugging close.
type holder struct {
	key    int
	closer io.Closer
}

// NewWatcher creates Watcher with various options.
func NewWatcher(options ...Option) (yama *Watcher) {
	w := &Watcher{
		signals: make(chan os.Signal, 1),
		done:    make(chan struct{}, 1),
	}

	s := &Settings{TimeOut: DefaultTimeout}
	for _, option := range options {
		option.Apply(s)
	}
	w.timeout = s.TimeOut
	w.closers = s.Closers

	signal.Notify(w.signals, s.Signals...)

	// The wait group will be marked done when a signal is observed or the
	// watcher receives done.
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-w.signals:
				return
			case <-w.done:
				return
			}
		}
	}()

	return w
}

// Wait until the configured signal occurs or the instance is closed.
func (w *Watcher) Wait() error {
	w.wg.Wait()
	return w.notify()
}

// Close the instance, notifying any registered closers. Can be called
// multiple times, but closers will only be called once.
func (w *Watcher) Close() error {
	select {
	case w.done <- struct{}{}:
	default:
	}
	return w.notify()
}

// Notify closers, ensuring they are only called once.
func (w *Watcher) notify() error {
	w.mux.Lock()
	closers := w.closers
	w.closers = nil
	w.mux.Unlock()

	count := len(closers)
	if count > 0 {
		return w.notifyClosers(closers...)
	}
	return nil
}

// notifyClosers calls all closers once and wait for them to finish with a
// channel.  If not all closers return within the timeout, returns an error
// with the tardy closers.
func (w *Watcher) notifyClosers(closers ...io.Closer) (err error) {

	count := len(closers)
	pending := make(map[int]holder)
	completed := make(chan holder, count)

	for i, closer := range closers {
		holder := holder{key: i, closer: closer}
		go func() {
			_ = holder.closer.Close()
			completed <- holder
		}()
		pending[i] = holder
	}

	// wait on channels for notifications
	timer := time.NewTimer(w.timeout)
	for {
		select {
		case <-timer.C:
			var uncompleted []io.Closer
			for _, h := range pending {
				uncompleted = append(uncompleted, h.closer)
			}
			return &TimedOut{Uncompleted: uncompleted}
		case closer := <-completed:
			delete(pending, closer.key)
			count--

			if count == 0 || len(pending) == 0 {
				return nil
			}
		}
	}
}
