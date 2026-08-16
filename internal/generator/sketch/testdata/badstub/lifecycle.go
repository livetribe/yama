//go:build yamainject

package badstub

import (
	"context"

	"github.com/google/wire"
)

// NewAppLifecycle declares two results. A lifecycle stub declares three.
func NewAppLifecycle(ctx context.Context) (*App, error) {
	panic(wire.Build(NewLogger, NewApp))
}
