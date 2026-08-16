//go:build yamainject

package valuerun

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewAppLifecycle builds the app from a wire.Value provider.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(wire.Value(Config{Env: "prod"}), NewApp))
}
