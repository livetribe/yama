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
	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// A CreateFailed is the item for a target package that the run could not read.
// It moves no file. It is not in the set of packages that Google Wire runs
// over. Complete returns the error that the read produced.
type CreateFailed struct {
	err error
}

var _ State = (*CreateFailed)(nil)

// PackagePath reports no directory. The run read no package here, so this
// state holds no directory to name.
func (f *CreateFailed) PackagePath() (path string, runWire bool) {
	return "", false
}

// Prepare returns this state. A CreateFailed exists before the Prepare loop
// runs. The loop reaches this state and makes no change to it.
func (f *CreateFailed) Prepare() State {
	return f
}

// Generate returns this state. A CreateFailed reaches the Generate loop.
// Google Wire wrote nothing for a package that the run never read.
func (f *CreateFailed) Generate() State {
	return f
}

// Complete returns the error that the read produced. It settles no file. The
// run moved no file in this package.
func (f *CreateFailed) Complete() error {
	return f.err
}

// A PrepareFailed is the item for a target package that could not take the
// intermediate files, or that could not set a generated file aside. It is not
// in the set of packages that Google Wire runs over. Complete puts back every
// name that the run moved. Complete also returns the error that Prepare
// produced.
type PrepareFailed struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	err           error
}

var _ State = (*PrepareFailed)(nil)

// PackagePath reports no directory. Google Wire loads every directory of the
// run at one time. This package holds a file that would fail that load.
func (f *PrepareFailed) PackagePath() (path string, runWire bool) {
	return "", false
}

// Prepare panics. The driver calls Prepare once for each item, and this state
// is what that one call returned.
func (f *PrepareFailed) Prepare() State {
	panic("should never reach here")
}

// Generate returns this state. The Generate loop reaches every item. This
// package took no part in the Google Wire run.
func (f *PrepareFailed) Generate() State {
	return f
}

// Complete settles the package's files. It returns the error that Prepare
// produced.
func (f *PrepareFailed) Complete() error {
	return settle(f.custodian, f.intermediates, f.err)
}

// A GenerateFailed is the item for a target package that failed after Google
// Wire ran. The load, the analysis, the render, or the write of that package
// failed. Complete puts back every name that the run moved. Complete also
// returns the error that Generate produced.
type GenerateFailed struct {
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	err           error
}

var _ State = (*GenerateFailed)(nil)

// PackagePath reports no directory. Google Wire already ran, and the driver
// reads this call only to build the set that Google Wire runs over.
func (f *GenerateFailed) PackagePath() (path string, runWire bool) {
	return "", false
}

// Prepare panics. The Prepare loop finished before this state existed.
func (f *GenerateFailed) Prepare() State {
	panic("should never reach here")
}

// Generate panics. The driver calls Generate once for each item, and this
// state is what that one call returned.
func (f *GenerateFailed) Generate() State {
	panic("should never reach here")
}

// Complete settles the package's files. It returns the error that Generate
// produced. That error names the package that it came from.
func (f *GenerateFailed) Complete() error {
	return settle(f.custodian, f.intermediates, f.err)
}
