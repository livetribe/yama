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

package work

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"l7e.io/yama/internal/generator/pkg"
)

// The construction keeps the names that Google Wire gave its imports. A stub
// file's import keeps its own name unless the output gives that name to another
// package.
func TestNameImports(t *testing.T) {
	cases := []struct {
		name        string
		stubs       []pkg.Import
		output      []pkg.Import
		taken       map[string]bool
		wantBlock   []pkg.Import
		wantRenamed map[string]string
	}{
		{
			name:      "an output entry keeps its name",
			output:    []pkg.Import{{Name: "template", Path: "text/template"}},
			wantBlock: []pkg.Import{{Name: "template", Path: "text/template"}},
		},
		{
			name:      "a stub entry that the output does not take keeps its name",
			stubs:     []pkg.Import{{Name: "config", Path: "example.com/config"}},
			wantBlock: []pkg.Import{{Name: "config", Path: "example.com/config"}},
		},
		{
			name:      "one entry stands for a package that both blocks import under one name",
			stubs:     []pkg.Import{{Name: "context", Path: "context"}},
			output:    []pkg.Import{{Name: "context", Path: "context"}},
			wantBlock: []pkg.Import{{Name: "context", Path: "context"}},
		},
		{
			name:   "a stub entry keeps a free name when the output names its package another way",
			stubs:  []pkg.Import{{Name: "cfg", Path: "example.com/config"}},
			output: []pkg.Import{{Name: "config", Path: "example.com/config"}},
			wantBlock: []pkg.Import{
				{Name: "config", Path: "example.com/config"},
				{Name: "cfg", Path: "example.com/config"},
			},
		},
		{
			name:  "a stub entry takes the name that the output gives its package",
			stubs: []pkg.Import{{Name: "template", Path: "html/template"}},
			output: []pkg.Import{
				{Name: "template", Path: "text/template"},
				{Name: "template2", Path: "html/template"},
			},
			wantBlock: []pkg.Import{
				{Name: "template", Path: "text/template"},
				{Name: "template2", Path: "html/template"},
			},
			wantRenamed: map[string]string{"template": "template2"},
		},
		{
			name:   "a stub entry takes the lowest free number when the output does not import its package",
			stubs:  []pkg.Import{{Name: "context", Path: "context"}},
			output: []pkg.Import{{Name: "context", Path: "go/build"}},
			wantBlock: []pkg.Import{
				{Name: "context", Path: "go/build"},
				{Name: "context2", Path: "context"},
			},
			wantRenamed: map[string]string{"context": "context2"},
		},
		{
			name:  "the number skips a name that the output holds",
			stubs: []pkg.Import{{Name: "context", Path: "context"}},
			output: []pkg.Import{
				{Name: "context", Path: "go/build"},
				{Name: "context2", Path: "example.com/context"},
			},
			wantBlock: []pkg.Import{
				{Name: "context", Path: "go/build"},
				{Name: "context2", Path: "example.com/context"},
				{Name: "context3", Path: "context"},
			},
			wantRenamed: map[string]string{"context": "context3"},
		},
		{
			name:   "the number skips a name that the file's scope holds",
			stubs:  []pkg.Import{{Name: "context", Path: "context"}},
			output: []pkg.Import{{Name: "context", Path: "go/build"}},
			taken:  map[string]bool{"context2": true},
			wantBlock: []pkg.Import{
				{Name: "context", Path: "go/build"},
				{Name: "context3", Path: "context"},
			},
			wantRenamed: map[string]string{"context": "context3"},
		},
		{
			name: "the number skips a name that a later stub entry holds",
			stubs: []pkg.Import{
				{Name: "context", Path: "context"},
				{Name: "context2", Path: "example.com/context"},
			},
			output: []pkg.Import{{Name: "context", Path: "go/build"}},
			wantBlock: []pkg.Import{
				{Name: "context", Path: "go/build"},
				{Name: "context3", Path: "context"},
				{Name: "context2", Path: "example.com/context"},
			},
			wantRenamed: map[string]string{"context": "context3"},
		},
		{
			name: "a second stub entry for one package takes the name that the first entry kept",
			stubs: []pkg.Import{
				{Name: "cfg", Path: "example.com/config"},
				{Name: "template", Path: "example.com/config"},
			},
			output: []pkg.Import{{Name: "template", Path: "text/template"}},
			wantBlock: []pkg.Import{
				{Name: "template", Path: "text/template"},
				{Name: "cfg", Path: "example.com/config"},
			},
			wantRenamed: map[string]string{"template": "cfg"},
		},
		{
			name:      "a stub entry keeps a name that the file's scope also holds",
			stubs:     []pkg.Import{{Name: "config", Path: "example.com/config"}},
			taken:     map[string]bool{"config": true},
			wantBlock: []pkg.Import{{Name: "config", Path: "example.com/config"}},
		},
		{
			name: "Google Wire's package and Yama's own two packages take no part",
			stubs: []pkg.Import{
				{Name: "wire", Path: pkg.WirePackagePath},
				{Name: "yama", Path: yamaPath},
				{Name: "rt", Path: rtPath},
			},
		},
		{
			name: "a blank import and a dot import take no part",
			stubs: []pkg.Import{
				{Name: ".", Path: "example.com/a/lib"},
				{Name: ".", Path: "example.com/b/lib"},
			},
			output: []pkg.Import{
				{Name: "_", Path: "github.com/lib/pq"},
				{Name: "_", Path: "github.com/mattn/go-sqlite3"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nameImports(c.stubs, c.output, c.taken)

			assert.Equal(t, c.wantBlock, got.block)

			if c.wantRenamed == nil {
				assert.Empty(t, got.renamed)

				return
			}

			assert.Equal(t, c.wantRenamed, got.renamed)
		})
	}
}

// A type in a signature names a package by the left side of a selector.
// requalify replaces those names and no other text.
func TestRequalify(t *testing.T) {
	cases := []struct {
		name  string
		typ   string
		names map[string]string
		want  string
	}{
		{
			name:  "a pointer type",
			typ:   "*server.System",
			names: map[string]string{"server": "server2"},
			want:  "*server2.System",
		},
		{
			name:  "a variadic type",
			typ:   "...yama.Option",
			names: map[string]string{"yama": "yama2"},
			want:  "...yama2.Option",
		},
		{
			name:  "a function type",
			typ:   "func(ctx context.Context) error",
			names: map[string]string{"context": "context2"},
			want:  "func(ctx context2.Context) error",
		},
		{
			name:  "two names that trade places",
			typ:   "map[text.Key]*template.Template",
			names: map[string]string{"text": "template", "template": "template2"},
			want:  "map[template.Key]*template2.Template",
		},
		{
			name:  "a generic type",
			typ:   "set.Of[config.Key]",
			names: map[string]string{"set": "set2", "config": "config2"},
			want:  "set2.Of[config2.Key]",
		},
		{
			name:  "a struct tag that holds a name and a dot",
			typ:   "struct{ F int \"a.b\" }",
			names: map[string]string{"a": "z"},
			want:  "struct{ F int \"a.b\" }",
		},
		{
			name:  "a type that names no mapped package",
			typ:   "*App",
			names: map[string]string{"server": "server2"},
			want:  "*App",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, requalify(c.typ, c.names))
		})
	}
}

// Each stub file can give Yama's own two packages a name of its own. Every such
// name takes the name that the lifecycle file gives the package.
func TestQualifiers(t *testing.T) {
	stubs := []pkg.Import{
		{Name: "rt", Path: rtPath},
		{Name: "runtime", Path: rtPath},
		{Name: "y", Path: yamaPath},
		{Name: "yama", Path: yamaPath},
		{Name: "template", Path: "html/template"},
	}
	names := naming{yama: "yama", rt: "rt2"}
	renamed := map[string]string{"template": "template2"}

	got := qualifiers(stubs, &names, renamed)

	want := map[string]string{"rt": "rt2", "runtime": "rt2", "y": "yama", "template": "template2"}
	assert.Equal(t, want, got)
}
