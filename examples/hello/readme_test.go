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

package hello_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/hello"
)

// licenseEnd holds the last line of the license header and the blank line
// after that line.
const licenseEnd = "// limitations under the License.\n\n"

// readmeFiles holds the paths of the files that README.md shows. In README.md,
// each file has a heading that holds only the file's path in backticks.
var readmeFiles = []string{"components.go", "client.go", "lifecycle.go", "cmd/hello/main.go"}

// TestReadmeMatchesTheSources asserts that README.md shows each file in
// readmeFiles. The code block under the heading of each file must be the
// content of that file without the license header.
func TestReadmeMatchesTheSources(t *testing.T) {
	blocks := readmeBlocks(t)

	for _, name := range readmeFiles {
		got, ok := blocks["`"+name+"`"]
		if !ok {
			t.Errorf("README.md has no code block under the heading `%s`", name)
			continue
		}

		source, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}

		want := string(source)
		if _, body, found := strings.Cut(want, licenseEnd); found {
			want = body
		}
		if got != want {
			t.Errorf("README.md shows a different %s\n got %q\nwant %q", name, got, want)
		}
	}
}

// TestReadmeShowsTheOutput starts and stops the lifecycle. It asserts that the
// code block under the Output heading in README.md is the output of the
// components.
func TestReadmeShowsTheOutput(t *testing.T) {
	blocks := readmeBlocks(t)

	want, ok := blocks["Output"]
	if !ok {
		t.Fatal("README.md has no code block under the Output heading")
	}

	var out bytes.Buffer
	_, lc, err := hello.NewLifecycle(&out)
	if err != nil {
		t.Fatalf("NewLifecycle: %v", err)
	}

	if err := lc.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	lc.Stop(t.Context())

	if got := out.String(); got != want {
		t.Fatalf("README.md shows different output\n got %q\nwant %q", got, want)
	}
}

// readmeBlocks returns the first fenced code block under each second-level
// heading in README.md. The key of each block is the text of its heading. A
// block does not include its fence lines.
func readmeBlocks(t *testing.T) map[string]string {
	t.Helper()

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	blocks := map[string]string{}
	var heading string
	var block strings.Builder
	inBlock := false
	for line := range strings.Lines(string(readme)) {
		switch {
		case inBlock && line == "```\n":
			inBlock = false
			if _, seen := blocks[heading]; !seen {
				blocks[heading] = block.String()
			}
		case inBlock:
			block.WriteString(line)
		case strings.HasPrefix(line, "```"):
			inBlock = true
			block.Reset()
		case strings.HasPrefix(line, "## "):
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
	}

	return blocks
}
