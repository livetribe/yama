package work

import (
	"context"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// A CreateFailed is the item for a target package that the run could not read.
// It moves no file. It stays out of the set of packages that Google Wire runs
// over. Complete returns the error that the read produced.
type CreateFailed struct {
	err error
}

var _ State = (*CreateFailed)(nil)

func (f *CreateFailed) PackagePath() (path string, ok bool) {
	return "", false
}

func (f *CreateFailed) Prepare(_ context.Context) State {
	return f
}

func (f *CreateFailed) Generate(_ context.Context) State {
	return f
}

// Complete returns the error that the read produced. It settles no file. The
// run moved none in this package.
func (f *CreateFailed) Complete(_ context.Context) error {
	return f.err
}

type PrepareFailed struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	err           error
}

var _ State = (*PrepareFailed)(nil)

func (f *PrepareFailed) PackagePath() (path string, ok bool) {
	return "", false
}

func (f *PrepareFailed) Prepare(_ context.Context) State {
	panic("should never reach here")
}

func (f *PrepareFailed) Generate(_ context.Context) State {
	// do nothing
	return f
}

// Complete settles the package's files and returns the error that Prepare
// produced.
func (f *PrepareFailed) Complete(_ context.Context) error {
	return settle(f.custodian, f.intermediates, f.err)
}

type GenerateFailed struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	err           error
}

var _ State = (*GenerateFailed)(nil)

func (f *GenerateFailed) PackagePath() (path string, ok bool) {
	panic("should never reach here")
}

func (f *GenerateFailed) Prepare(_ context.Context) State {
	panic("should never reach here")
}

func (f *GenerateFailed) Generate(_ context.Context) State {
	panic("should never reach here")
}

// Complete settles the package's files and returns the error that Generate
// produced.
func (f *GenerateFailed) Complete(_ context.Context) error {
	return settle(f.custodian, f.intermediates, f.err)
}
