// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
