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

package rt

import (
	"context"
	"sync"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/bridge"
)

var _ yama.Lifecycle = (*lifecycle)(nil)

// lifecycle is the private yama.Lifecycle implementation. It holds no level list;
// startPass and shutdown are generated functions that encode ordering directly.
// lifecycle adds only the cross-cutting policy around them: a cancelable startup
// context, startup-failure cleanup, and once-only shutdown.
type lifecycle struct {
	startPass func(context.Context) error
	shutdown  func(context.Context)

	stopOnce sync.Once
}

// NewLifecycle builds a yama.Lifecycle from a generated start pass and a generated
// shutdown sequence. startPass runs the Start pass and returns the first failure;
// shutdown runs the quiesce pass followed by the teardown pass.
func NewLifecycle(startPass func(context.Context) error, shutdown func(context.Context)) yama.Lifecycle {
	return &lifecycle{startPass: startPass, shutdown: shutdown}
}

// Start runs the start pass under a cancelable child of ctx. On failure it cancels
// that context, runs the shutdown sequence scoped to the components that started,
// and returns ErrStartFailed — never the component's own error.
func (l *lifecycle) Start(ctx context.Context) error {
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := l.startPass(sctx); err != nil {
		cancel()
		l.Stop(ctx)
		return yama.ErrStartFailed
	}

	return nil
}

// Stop runs the shutdown sequence once. Repeated or concurrent calls, and a call
// after Start already ran startup-failure cleanup, observe the same single run.
func (l *lifecycle) Stop(ctx context.Context) {
	l.stopOnce.Do(func() {
		l.shutdown(ctx)
	})
}

// Assemble applies the construction options and returns the interceptor chains
// together with the begin and end boundary runners. Generated code wraps its
// graph components through the returned chains.
func Assemble(opts ...yama.Option) (chains *Chains, begin, end *Boundary) {
	cfg := &bridge.Config{}
	for _, o := range opts {
		o.Apply(cfg)
	}

	chains = NewChains(cfg.Interceptors)
	begin = newBoundary(cfg.BeginComponents, chains)
	end = newBoundary(cfg.EndComponents, chains)

	return chains, begin, end
}
