//go:build yamainject

package badwire

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewAppLifecycle names no provider of *DB, so Google Wire rejects it.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewApp))
}
