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

package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"l7e.io/yama/v2/internal/generator/pkg"
)

func TestNameIn(t *testing.T) {
	resolved := map[string]string{"gopkg.in/yaml.v3": "yaml"}

	cases := []struct {
		name string
		imp  pkg.Import
		want string
	}{
		{"the alias that the file gave", pkg.Import{Name: "applib", Path: "example.com/app/lib"}, "applib"},
		{"the name that the package declares", pkg.Import{Path: "gopkg.in/yaml.v3"}, "yaml"},
		{"the alias over the declared name", pkg.Import{Name: "y", Path: "gopkg.in/yaml.v3"}, "y"},
		{"a guess from the path", pkg.Import{Path: "example.com/app/lib"}, "lib"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.imp.NameIn(resolved))
		})
	}
}

func TestNameInWithoutResolvedNames(t *testing.T) {
	imp := pkg.Import{Path: "gopkg.in/yaml.v3"}

	assert.Equal(t, "yaml", imp.NameIn(nil))
}

func TestPackageName(t *testing.T) {
	cases := []struct {
		name  string
		alias string
		path  string
		want  string
	}{
		{"the alias wins", "applib", "example.com/app/lib", "applib"},
		{"the last element", "", "example.com/app/lib", "lib"},
		{"the element before a major version", "", "l7e.io/yama/v2", "yama"},
		{"a gopkg.in version suffix", "", "gopkg.in/yaml.v3", "yaml"},
		{"a gopkg.in version suffix on a hyphenated name", "", "gopkg.in/go-yaml.v2", "go-yaml"},
		{"a single element", "", "context", "context"},
		{"a host that is the whole path", "", "example.com", "example.com"},
		{"an element that only looks versioned", "", "example.com/app/lib.go", "lib.go"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, pkg.PackageName(c.alias, c.path))
		})
	}
}

func TestQualifiers(t *testing.T) {
	src := []byte(`package app

// yaml names nothing here.
func New(n *yaml.Node) *App {
	app := &App{}
	app.Field = lib.Value

	return app
}
`)

	used := pkg.Qualifiers(src)

	assert.True(t, used["yaml"], "a qualified type reads as a package")
	assert.True(t, used["lib"], "a qualified value reads as a package")
	assert.True(t, used["app"], "a selection on a local reads the same way")
	assert.False(t, used["Field"], "a selected name is not a package")
	assert.False(t, used["Node"], "a selected type is not a package")
}

func TestQualifiersTakesNothingFromSourceThatDoesNotParse(t *testing.T) {
	assert.Empty(t, pkg.Qualifiers([]byte("this is not go")))
}

func TestCheckImports(t *testing.T) {
	cases := []struct {
		name  string
		block []pkg.Import
		names map[string]string
		want  string
	}{
		{
			name:  "names that each refer to one path",
			block: []pkg.Import{{Path: "context"}, {Path: "example.com/app/lib"}},
		},
		{
			name:  "the same import stated by two files",
			block: []pkg.Import{{Path: "context"}, {Path: "context"}},
		},
		{
			name:  "one name that two paths answer to",
			block: []pkg.Import{{Path: "example.com/a/lib"}, {Path: "example.com/b/lib"}},
			want:  "the name lib refers to",
		},
		{
			name:  "a conflict that an alias created",
			block: []pkg.Import{{Path: "example.com/a/lib"}, {Name: "lib", Path: "example.com/b/other"}},
			want:  "the name lib refers to",
		},
		{
			name:  "a conflict that an alias settled",
			block: []pkg.Import{{Path: "example.com/a/lib"}, {Name: "blib", Path: "example.com/b/lib"}},
		},
		{
			// Two paths that a guess reads as one name can declare names of
			// their own that differ. The check reads the name that a package
			// declares.
			name:  "the name that each package declares",
			block: []pkg.Import{{Path: "example.com/a/yaml.v3"}, {Path: "example.com/b/yaml.v3"}},
			names: map[string]string{"example.com/a/yaml.v3": "yaml", "example.com/b/yaml.v3": "yamlv3"},
		},
		{
			name: "a package that imports nothing",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := pkg.CheckImports(c.block, c.names)

			if c.want == "" {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)

			for _, imp := range c.block {
				assert.Contains(t, err.Error(), imp.Path)
			}
		})
	}
}

func TestImportsIn(t *testing.T) {
	src := []byte(`package app

import (
	"context"
	applib "example.com/app/lib"
)
`)

	assert.Equal(t, []pkg.Import{
		{Path: "context"},
		{Name: "applib", Path: "example.com/app/lib"},
	}, pkg.ImportsIn(src))
}

func TestImportsInTakesNothingFromSourceThatDoesNotParse(t *testing.T) {
	assert.Empty(t, pkg.ImportsIn([]byte("this is not go")))
}
