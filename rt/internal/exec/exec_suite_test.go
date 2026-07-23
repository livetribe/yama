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

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
)

// ginkgoInterrupts tops the stack of Ginkgo's signal-watching goroutine, which
// runs for the life of the test binary.
const ginkgoInterrupts = "github.com/onsi/ginkgo/v2/internal/" +
	"interrupt_handler.(*InterruptHandler).registerForInterrupts.func2"

// TestMain runs the specs and then fails the package if any goroutine outlived
// them.
func TestMain(m *testing.M) {
	ignoreGinkgo := goleak.IgnoreTopFunction(ginkgoInterrupts)

	goleak.VerifyTestMain(m, ignoreGinkgo)
}

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "rt/internal/exec")
}

// inertComponent implements none of the three lifecycle capabilities.
type inertComponent struct{}

// capture is one record together with the context it was emitted under.
type capture struct {
	ctx context.Context
	rec slog.Record
}

// captureHandler collects every record routed through it, and the context that
// carried it. Specs that install one replace the slog default, which is
// process-global: they must not run in parallel with one another.
type captureHandler struct {
	mu   sync.Mutex
	caps []capture
}

var _ slog.Handler = (*captureHandler)(nil)

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // Handle's by-value slog.Record parameter is fixed by slog.Handler.
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.caps = append(h.caps, capture{ctx: ctx, rec: r.Clone()})

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// records returns a snapshot of everything captured so far.
func (h *captureHandler) records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	var recs []slog.Record
	for i := range h.caps {
		recs = append(recs, h.caps[i].rec)
	}

	return recs
}

// contexts returns the context each captured record was emitted under, in the
// same order as records.
func (h *captureHandler) contexts() []context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()

	var ctxs []context.Context
	for i := range h.caps {
		ctxs = append(ctxs, h.caps[i].ctx)
	}

	return ctxs
}

// attrs flattens a record's attributes into a lookup keyed by attribute name.
//
//nolint:gocritic // slog.Record is designed to be passed by value.
func attrs(r slog.Record) map[string]slog.Value {
	m := make(map[string]slog.Value, r.NumAttrs())

	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value
		return true
	})

	return m
}

// captureSlog installs a capturing handler as the slog default for the current
// spec and restores the previous default afterwards.
func captureSlog() *captureHandler {
	h := &captureHandler{}

	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	DeferCleanup(func() { slog.SetDefault(prev) })

	return h
}

// expired returns a context whose deadline has already passed, so any operation
// run under it returns late.
func expired() context.Context {
	//nolint:gosec // cancel is called via DeferCleanup below.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	DeferCleanup(cancel)

	return ctx
}
