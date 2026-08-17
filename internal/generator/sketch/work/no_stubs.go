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
