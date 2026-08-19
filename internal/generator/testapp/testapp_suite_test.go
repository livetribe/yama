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

package testapp_test

import (
	"log/slog"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTestApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/generator/testapp")
}

// The specs drive a failing start to prove the generated constructor and the
// runtime tear the graph down together; no spec here asserts on what gets
// logged, so the slog default is silenced for the whole run rather than let
// that noise print on every green pass.
var _ = BeforeSuite(func() {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	DeferCleanup(func() { slog.SetDefault(prev) })
})
