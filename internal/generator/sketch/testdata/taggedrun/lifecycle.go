//go:build yamainject

package taggedrun

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewAppLifecycle builds the app and its lifecycle.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewDB, NewApp))
}
