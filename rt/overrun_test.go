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

package rt_test

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/rt"
)

// captureHandler records every slog record routed through it.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // Handle's by-value slog.Record parameter is fixed by the slog.Handler interface.
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) list() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

// captureSlog installs a capturing handler as the slog default logger for the test
// and restores the previous default on cleanup.
func captureSlog(t *testing.T) *captureHandler {
	t.Helper()
	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func attrsOf(r *slog.Record) map[string]slog.Value {
	m := make(map[string]slog.Value, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value
		return true
	})
	return m
}

// expiredDeadline returns a context whose deadline has already passed, so any
// operation returns after it and an overrun is reported deterministically.
func expiredDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	t.Cleanup(cancel)
	return ctx
}

// TestOverrunReportedOncePerComponent proves the built-in overrun interceptor
// reports a per-component overrun exactly once, attributed to the component, and
// waits for the operation to complete rather than abandoning it. The deadline
// fires mid-operation, before the component's Start returns.
func TestOverrunReportedOncePerComponent(t *testing.T) {
	h := captureSlog(t)
	comp := &fakeComponent{name: "kafka-consumer", sleep: 50 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := rt.NewChains(nil).WrapStart(comp).Start(ctx)
	require.NoError(t, err)

	assert.True(t, comp.done(), "the operation must run to completion; the framework does not abandon it")
	assert.Equal(t, 1, comp.starts())

	records := h.list()
	require.Len(t, records, 1, "expected exactly one overrun record")
	r := records[0]
	assert.Equal(t, slog.LevelWarn, r.Level, "overrun should log at Warn")
	assert.Contains(t, strings.ToLower(r.Message), "deadline")

	attrs := attrsOf(&r)
	assert.Equal(t, "kafka-consumer", attrs["component"].String(), "overrun should be attributed to the component")
	require.Equal(t, slog.KindDuration, attrs["overrun"].Kind(), "overrun should carry an elapsed-vs-deadline delta")
	assert.Positive(t, attrs["overrun"].Duration())
}

// TestNoOverrunWithoutDeadline confirms nothing is logged when the caller's
// context carries no deadline.
func TestNoOverrunWithoutDeadline(t *testing.T) {
	h := captureSlog(t)
	comp := &fakeComponent{name: "component"}

	require.NoError(t, rt.NewChains(nil).WrapStart(comp).Start(context.Background()))
	assert.Empty(t, h.list(), "no deadline means no overrun report")
}

// TestNoOverrunWhenWithinDeadline confirms an operation that returns before the
// deadline produces no overrun record.
func TestNoOverrunWhenWithinDeadline(t *testing.T) {
	h := captureSlog(t)
	comp := &fakeComponent{name: "component"}

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	require.NoError(t, rt.NewChains(nil).WrapStart(comp).Start(ctx))
	assert.Empty(t, h.list(), "an operation within its deadline must not report an overrun")
}

// TestOverrunIdentifiesComponentByType confirms a component without a fmt.Stringer
// identity is attributed by its %T type.
func TestOverrunIdentifiesComponentByType(t *testing.T) {
	h := captureSlog(t)
	comp := &stopperOnly{}

	rt.NewChains(nil).WrapStop(comp).Stop(expiredDeadline(t))

	records := h.list()
	require.Len(t, records, 1)
	assert.Equal(t, "*rt_test.stopperOnly", attrsOf(&records[0])["component"].String())
}
