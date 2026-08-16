package emit_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEmit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/generator/sketch/emit")
}
