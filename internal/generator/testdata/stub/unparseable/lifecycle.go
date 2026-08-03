//go:build yamainject

package unparseable

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewLifecycle is missing a closing paren, so this file does not parse.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewApp)
}
