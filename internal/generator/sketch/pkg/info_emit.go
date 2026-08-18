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

package pkg

// LifecycleImports returns every import that the lifecycle file can carry, each
// under the name that refers to it, and without a repeat. The set holds the
// stub files' own imports, which a signature names. It also holds Google Wire's
// own imports, which the construction names.
//
// Google Wire itself is not in the set. A stub imports it for its Build call,
// and the lifecycle file calls nothing from it.
//
// One path reaches the set two times when a stub file and Google Wire's output
// both state it. One entry then holds both. One path under two names also
// reaches the set two times, and both entries stay. A signature can name a
// package by the alias that a stub gave it, and the construction can name the
// same package by the name that Google Wire used.
func (info *Info) LifecycleImports() []Import {
	all := info.AllImports()

	out := make([]Import, 0, len(all))
	seen := make(map[Import]bool, len(all))

	for _, imp := range all {
		if imp.Path == WirePackagePath {
			continue
		}

		name := info.NameOf(imp)

		entry := Import{Name: name, Path: imp.Path}
		if seen[entry] {
			continue
		}

		seen[entry] = true
		out = append(out, entry)
	}

	return out
}
