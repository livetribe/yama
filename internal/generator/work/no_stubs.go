// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package work

// A NoStubs is the item for a target package that declares no lifecycle stub.
// Google Wire does not run over this package. Yama writes no lifecycle file
// for it. Every phase makes no change to the directory.
type NoStubs struct{}

var _ State = (*NoStubs)(nil)

// PackagePath reports no directory. This package declares no stub, so Google
// Wire has nothing to generate here.
func (ns *NoStubs) PackagePath() (path string, runWire bool) {
	return "", false
}

// Prepare moves no file. Google Wire writes only in a directory that the run
// names for it. The run does not name this directory.
func (ns *NoStubs) Prepare() State {
	return ns
}

// Generate reads no file. Google Wire's output in the directory belongs to the
// application.
func (ns *NoStubs) Generate() State {
	return ns
}

// Complete settles no file. This run moved no file here.
func (ns *NoStubs) Complete() error {
	return nil
}
