//go:build yamainject

package tagged

import (
	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewLifecycle orchestrates the graph NewDep and NewRoot build. NewDep is only
// visible under the "special" build tag.
func NewLifecycle(opts ...yama.Option) (*Root, yama.Lifecycle, error) {
	panic(wire.Build(NewDep, NewRoot))
}
