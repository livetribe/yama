//go:build yamainject

package wirerun

import (
	"context"

	"github.com/google/wire"
	"gopkg.in/yaml.v3"

	yama "l7e.io/yama/v2"
)

// NewAppLifecycle builds the app and its lifecycle. The yaml parameter is here
// for its import: gopkg.in/yaml.v3 declares the package yaml, which no rule
// over the path alone produces.
func NewAppLifecycle(ctx context.Context, node *yaml.Node, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewDB, NewApp))
}
