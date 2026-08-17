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

type State interface {
	PackagePath() (path string, ok bool)

	// Prepare is called before Google Wire is called.
	Prepare() State

	Generate() State

	// Complete is called to perform any cleanup duties after all the
	// lifecycle_gen.go files have been created.
	Complete() error
}

type Items []State

func (items Items) Paths() []string {
	var paths []string

	for _, item := range items {
		if path, ok := item.PackagePath(); ok {
			paths = append(paths, path)
		}
	}

	return paths
}
