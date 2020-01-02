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
	"time"
)

// Settings holds information needed to construct an instance of Watcher.
type Settings struct {
	Signals []os.Signal
	TimeOut time.Duration
	Closers []io.Closer
}

// A Option is an option for a Watcher watcher.
type Option interface {
	Apply(*Settings)
}

// WatchingSignals returns an Option that specifies the OS signals to capture.
func WatchingSignals(signals ...os.Signal) Option {
	return watchingSignals{signals: signals}
}

type watchingSignals struct{ signals []os.Signal }

func (w watchingSignals) Apply(o *Settings) {
	o.Signals = w.signals
}

// WithTimeout returns an Option that specifies the timeout used when calling
// closers when a signal is captured or the Watcher instance is closed.  The
// default timeout is ten seconds.
func WithTimeout(timeout time.Duration) Option {
	return withTimeout{timeout: timeout}
}

type withTimeout struct{ timeout time.Duration }

func (w withTimeout) Apply(o *Settings) {
	o.TimeOut = w.timeout
}

// WithClosers returns an Option that specifies the closers to call when a
// signal is captured or the Watcher instance is closed.  Closers are only
// called once.
func WithClosers(closers ...io.Closer) Option {
	return withClosers{closers: closers}
}

type withClosers struct{ closers []io.Closer }

func (w withClosers) Apply(o *Settings) {
	o.Closers = w.closers
}
