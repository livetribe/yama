// Copyright the original author or authors.
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

package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"l7e.io/yama/internal/generator/pkg"
)

// Google Wire chooses the names of its own imports. Its output can give a name
// that a stub file uses to another path. The package takes that block as it
// stands.
func TestTakeOutputImportsFromTakesANameThatAStubFileGivesAnotherPath(t *testing.T) {
	info := pkg.NewInfo(t.TempDir())
	info.TakeStubFile("app", nil, []pkg.Import{{Name: "cfg", Path: "example.com/a/config"}})

	info.TakeOutputImportsFrom([]byte("package app\n\nimport cfg \"example.com/b/config\"\n"))

	assert.Equal(t, []pkg.Import{{Name: "cfg", Path: "example.com/b/config"}}, info.OutputImports())
}

func TestTakeOutputImportsFromTakesNothingFromOutputThatDoesNotParse(t *testing.T) {
	info := pkg.NewInfo(t.TempDir())

	info.TakeOutputImportsFrom([]byte("this is not go"))

	assert.Empty(t, info.OutputImports())
}
