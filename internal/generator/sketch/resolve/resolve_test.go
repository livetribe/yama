package resolve_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"l7e.io/yama/v2/internal/generator/sketch/resolve"
)

func TestNamesReadsTheNameAPackageDeclares(t *testing.T) {
	names := resolve.Names(".", []string{"context"})

	assert.Equal(t, "context", names["context"])
}

// A path that ends in a major version holds no package name. The element before
// it does, and every guess from the path alone has to know that rule.
func TestNamesReadsAPathThatEndsInAMajorVersion(t *testing.T) {
	names := resolve.Names(".", []string{"l7e.io/yama/v2"})

	assert.Equal(t, "yama", names["l7e.io/yama/v2"])
}

// gopkg.in/yaml.v3 declares the package yaml. No rule over the path alone
// produces that name, so a caller that guesses gets it wrong.
func TestNamesReadsAPathThatNoRuleCouldGuess(t *testing.T) {
	names := resolve.Names(".", []string{"gopkg.in/yaml.v3"})

	assert.Equal(t, "yaml", names["gopkg.in/yaml.v3"])
}

func TestNamesReadsEveryPathItWasGiven(t *testing.T) {
	paths := []string{"context", "gopkg.in/yaml.v3", "l7e.io/yama/v2"}

	names := resolve.Names(".", paths)

	assert.Equal(t, map[string]string{
		"context":          "context",
		"gopkg.in/yaml.v3": "yaml",
		"l7e.io/yama/v2":   "yama",
	}, names)
}

func TestNamesReturnsAnEmptyMapForNoPaths(t *testing.T) {
	names := resolve.Names(".", nil)

	assert.Empty(t, names)
}

// A path the toolchain cannot read leaves no entry. The caller keeps whatever
// it already had for that path.
func TestNamesLeavesOutAPathItCannotRead(t *testing.T) {
	names := resolve.Names(".", []string{"example.com/absent"})

	assert.NotContains(t, names, "example.com/absent")
}
