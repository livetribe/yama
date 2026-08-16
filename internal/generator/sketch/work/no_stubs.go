package work

import "context"

// A NoStubs is the item for a target package that declares no lifecycle stub.
// Google Wire does not run over it, and it writes no lifecycle file. Every
// phase leaves the directory as it found it.
type NoStubs struct{}

var _ State = (*NoStubs)(nil)

func (ns *NoStubs) PackagePath() (path string, ok bool) {
	return "", false
}

// Prepare moves no file. Google Wire writes only in a directory that the run
// names for it, and the run does not name this one.
func (ns *NoStubs) Prepare(_ context.Context) State {
	return ns
}

// Generate reads no file. Google Wire's output in the directory belongs to the
// application.
func (ns *NoStubs) Generate(_ context.Context) State {
	return ns
}

// Complete settles no file. This run moved none.
func (ns *NoStubs) Complete(_ context.Context) error {
	return nil
}
