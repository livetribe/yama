//go:build yamainject

package setrun

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
	"l7e.io/yama/v2/internal/generator/sketch/testdata/setlib"
)

// NewAppLifecycle names a provider set. It never names the package that
// declares the provider in that set.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(setlib.ProviderSet, NewApp))
}
