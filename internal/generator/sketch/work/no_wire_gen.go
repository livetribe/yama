package work

import (
	"context"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

type NoWireGen struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
}

var _ State = (*NoWireGen)(nil)

func (n *NoWireGen) PackagePath() (path string, ok bool) {
	panic("should never reach here")
}

func (n *NoWireGen) Prepare(_ context.Context) State {
	panic("should never reach here")
}

func (n *NoWireGen) Generate(_ context.Context) State {
	panic("should never reach here")
}

// Complete settles the package's files. Google Wire generated nothing here, so
// the package owes the run no error of its own.
func (n *NoWireGen) Complete(_ context.Context) error {
	return settle(n.custodian, n.intermediates, nil)
}
