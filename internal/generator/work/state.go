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

// A State is one target package at one point of the run. Each phase returns a
// State. A phase that fails returns a State of a different type. The type of
// the item is therefore the record of what happened to that package.
type State interface {
	// PackagePath returns the directory to name to Google Wire. It also
	// returns whether to name that directory at all. A state that Google
	// Wire does not run over reports no directory and false.
	PackagePath() (path string, runWire bool)

	// Prepare runs before the run calls Google Wire. It puts the intermediate
	// files in the package's directory. It also sets both generated files
	// aside.
	Prepare() State

	// Generate runs after the run calls Google Wire. It reads Google Wire's
	// output for this package. Then it writes the lifecycle file.
	Generate() State

	// Complete settles the package's files. It also takes the intermediate
	// files out of the directory. It returns this package's error. A failure
	// of any phase reaches the caller through this call.
	Complete() error
}

// Items are the work items of one run, one for each target package. The driver
// holds them in the order that the run resolved their directories.
type Items []State

// Paths returns the directory of each item that Google Wire runs over.
func (items Items) Paths() []string {
	var paths []string

	for _, item := range items {
		if path, runWire := item.PackagePath(); runWire {
			paths = append(paths, path)
		}
	}

	return paths
}
