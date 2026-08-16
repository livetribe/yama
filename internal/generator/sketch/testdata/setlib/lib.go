// Package setlib states a provider set. A stub names the set, and never the
// package that declares the provider in it.
package setlib

import (
	"github.com/google/wire"

	"l7e.io/yama/v2/internal/generator/sketch/testdata/setother"
)

var ProviderSet = wire.NewSet(setother.NewThing)
