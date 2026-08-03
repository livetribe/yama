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

package testapp

import (
	"strings"
	"sync"
)

// Recorder collects the lifecycle events the components report, in the order
// they happen. A level's members run concurrently, so a spec that spans one
// level compares sets rather than sequences.
type Recorder struct {
	mu     sync.Mutex
	events []string
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Record appends one event.
func (r *Recorder) Record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)
}

// Events returns the events recorded so far.
func (r *Recorder) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.events...)
}

// Matching returns the recorded events whose name carries the given prefix.
func (r *Recorder) Matching(prefix string) []string {
	var matched []string
	for _, event := range r.Events() {
		if strings.HasPrefix(event, prefix) {
			matched = append(matched, event)
		}
	}

	return matched
}
