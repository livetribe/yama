// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEmit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/generator/emit")
}
