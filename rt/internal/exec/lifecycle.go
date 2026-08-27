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

package exec

//go:generate go run go.uber.org/mock/mockgen -source=lifecycle.go -destination=../mocks/exec_mocks.go -package=mocks

import (
	"context"
	"sync"

	"l7e.io/yama"
)

// CompleteLifecycle is the union of the three capabilities and the Release
// operation. Everything the lifecycle drives — a component, a component paired
// with its cleanup, a whole level — satisfies it, and each composes with the
// others uniformly. An element that does not participate in a pass implements
// that pass as a no-op rather than being absent.
type CompleteLifecycle interface {
	yama.Starter
	yama.Quiescer
	yama.Stopper

	// Release runs the element's Google Wire cleanup and nothing else. The
	// teardown pass calls Release on each level that startup did not reach.
	Release(ctx context.Context)
}

type lifecycleState int

const (
	// stateUnknown is the zero value and is never valid for a constructed
	// lifecycle; reaching it means the value was not built by NewLifecycle.
	stateUnknown lifecycleState = iota
	stateStopped
	stateStarted
	stateError
)

// lifecycle is the private yama.Lifecycle implementation. It holds the levels in
// dependency order; startup walks them forward and shutdown walks them back.
// The mutex serializes Start against Stop, so a Stop issued while a Start is in
// flight waits for that Start to finish.
type lifecycle struct {
	levels []Level

	mu    sync.Mutex
	state lifecycleState
}

var _ yama.Lifecycle = (*lifecycle)(nil)

func NewLifecycle(levels []Level) yama.Lifecycle {
	return &lifecycle{
		levels: levels,
		state:  stateStopped,
	}
}

// Start runs the levels in dependency order. The first level to fail stops the
// traversal: everything reached up to and including that level is torn down, and
// Start reports ErrStartFailed rather than the component's own error. Calling
// Start on an already-started lifecycle is a no-op, and calling it after Stop
// runs the levels again.
//
// A context already canceled when Start is called fails the start before any
// level runs; nothing is torn down and the lifecycle stays startable under a
// live context.
func (l *lifecycle) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch l.state {
	case stateUnknown:
		panic("uninitialized lifecycle")
	case stateStarted:
		return nil
	case stateError:
		return yama.ErrStartFailed
	case stateStopped:
	}

	if ctx.Err() != nil {
		return yama.ErrStartFailed
	}

	for i := 0; i < len(l.levels); i++ {
		level := l.levels[i]
		if err := level.Start(ctx); err != nil {
			l.stop(ctx, i)
			l.state = stateError
			return yama.ErrStartFailed
		}
	}

	l.state = stateStarted

	return nil
}

// Stop runs the shutdown sequence over every level. It is idempotent: a repeated
// Stop, or a Stop after startup failure already tore the graph down, does
// nothing.
func (l *lifecycle) Stop(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch l.state {
	case stateUnknown:
		panic("uninitialized lifecycle")
	case stateStopped, stateError:
		return
	case stateStarted:
	}

	l.stop(ctx, len(l.levels)-1)

	l.state = stateStopped
}

// stop runs the quiesce pass over levels level..0. It then runs the teardown
// pass over every level in reverse. The passes are separate traversals, so
// every quiesce completes before any teardown begins. Startup did not reach
// the levels above level. Those levels run only their cleanups.
func (l *lifecycle) stop(ctx context.Context, level int) {
	for i := level; 0 <= i; i-- {
		l.levels[i].Quiesce(ctx)
	}

	for i := len(l.levels) - 1; level < i; i-- {
		l.levels[i].Release(ctx)
	}

	for i := level; 0 <= i; i-- {
		l.levels[i].Stop(ctx)
	}
}
