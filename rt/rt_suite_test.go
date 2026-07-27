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
	"io"
	"log/slog"
	"testing"

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

func TestRT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "rt")
}

// The contract suite deliberately drives components through panicking and
// failing paths to prove the runtime recovers and reports them; no spec here
// asserts on what gets logged, so the slog default is silenced for the whole
// run rather than let recovery noise print to stderr on every green pass.
var _ = BeforeSuite(func() {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	DeferCleanup(func() { slog.SetDefault(prev) })
})

// verifyNoLeaks fails the current spec if a goroutine that started during it is
// still running when it ends. Goroutines already running at the call are not
// counted, so the report names only what this spec left behind. Call it first in
// a spec, so its check runs after every other cleanup.
func verifyNoLeaks() {
	existing := goleak.IgnoreCurrent()

	DeferCleanup(func() {
		goleak.VerifyNone(GinkgoT(), existing)
	})
}
