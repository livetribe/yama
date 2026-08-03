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

package generator

// Generator holds the resolved configuration for a generation run. Each of
// Google Wire's flag forms is projected from Options once, so a run that touches
// several packages derives each one a single time.
type Generator struct {
	// wireGenName is the file Google Wire writes: the file a run looks for,
	// preserves, and removes.
	wireGenName string

	// wireArgs is the `wire gen` flag list handed to the tool.
	wireArgs []string

	// discoveryBuildFlags load the packages Wire will generate for, under the same
	// tags Wire's own loader uses.
	discoveryBuildFlags []string

	// stubBuildFlags load a package's lifecycle stubs, under the tag that makes
	// them visible.
	stubBuildFlags []string

	// parseBuildFlags load Wire's generated output, without the wireinject tag that
	// makes its injector visible.
	parseBuildFlags []string
}

// NewGenerator resolves opts into the values a generation run reads, projecting
// each of Wire's flag forms once up front.
func NewGenerator(opts Options) *Generator {
	return &Generator{
		wireGenName:         opts.wireGenName(),
		wireArgs:            opts.wireArgs(),
		discoveryBuildFlags: opts.discoveryBuildFlags(),
		stubBuildFlags:      opts.stubBuildFlags(),
		parseBuildFlags:     opts.parseBuildFlags(),
	}
}
