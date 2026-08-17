// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
