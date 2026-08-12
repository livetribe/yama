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

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// wirePrefix begins every line Google Wire logs, and so distinguishes Wire's own
// diagnostics from the go command's.
const wirePrefix = "wire:"

// wireWroteMarker introduces the path in Google Wire's per-output success line,
// `wire: <pkgPath>: wrote <path>`. wireDiagnostics drops the lines carrying it.
const wireWroteMarker = ": wrote "

// wireInjectMarker introduces the injector's name in Google Wire's per-injector
// diagnostic, `inject <name>: <reason>`. wireDiagnostics rewrites the name that
// follows it.
const wireInjectMarker = "inject "

// runWire runs `go tool wire gen` once over patterns, from wd, and returns
// everything Wire logged.
//
// One invocation from the caller's own directory is how Google Wire runs itself,
// and matching that is what keeps behavior identical: a relative -header_file
// resolves against wd exactly as Wire resolves it, and Wire's diagnostics arrive
// in Wire's own order rather than reordered by a per-package loop.
//
// It overwrites any Wire output under those patterns, because that is what
// invoking Wire does, and it is deliberately unexported: EmitAll is the only way
// in, so the destructive step is always wrapped in the scopes that preserve the
// caller's files.
//
// derived names the injectors Yama derived for this run, which is what lets a
// diagnostic name the stub the application wrote. Pass nil when a run derives
// none.
//
// A Wire input problem surfaces as a *ToolError naming Wire's own
// diagnostic. An inability to launch the tool surfaces as a *ToolchainError.
// The two are distinct so a build failure points at the right cause.
func (g *Generator) runWire(ctx context.Context, wd string, patterns, derived []string) error {
	var stdout, stderr bytes.Buffer

	args := append([]string{"tool", "wire", "gen"}, g.wireArgs...)
	args = append(args, patterns...)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = wd
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}

	logged := strings.TrimSpace(stderr.String() + stdout.String())

	diagnostics := wireDiagnostics(logged, derived)

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && wireReported(logged) {
		return &ToolError{Dir: wd, Stderr: diagnostics, Err: err}
	}

	return &ToolchainError{Stderr: diagnostics, Err: err}
}

// wireDiagnostics is Wire's output with its per-output success lines removed
// and each name in derived replaced by the stub name that it was derived
// from.
//
// Wire reports `wrote <path>` for each file that it produced, but those
// files are transient here and gone by the time anything is printed. If the
// lines passed through, the caller would learn that Yama wrote files that it
// had already deleted, so only the diagnostics that remain true are kept.
//
// Wire names the injector that it rejects, and that name is one Yama
// invented. A rewrite applies to the name that follows Wire's own injector
// marker, and to nothing else. Every other text in the same diagnostic keeps
// the characters that it has, including a path that carries the reserved
// prefix: the transient file is named for the tag that it declares, so a
// stub named for that tag too derives an injector with a name that is the
// whole stem of that file.
func wireDiagnostics(logged string, derived []string) string {
	var kept []string
	for _, line := range strings.Split(logged, "\n") {
		if !strings.Contains(line, wireWroteMarker) {
			kept = append(kept, line)
		}
	}

	joined := strings.Join(kept, "\n")
	for _, name := range derived {
		stub := strings.TrimPrefix(name, derivedPrefix)
		joined = strings.ReplaceAll(joined, wireInjectMarker+name, wireInjectMarker+stub)
	}

	return strings.TrimSpace(joined)
}

// wireReported reports whether the output carries a Google Wire diagnostic
// (a line beginning "wire:"), which distinguishes Wire rejecting the input from
// the go command failing to launch the Wire tool at all.
func wireReported(diagnostic string) bool {
	for _, line := range strings.Split(diagnostic, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), wirePrefix) {
			return true
		}
	}

	return false
}

// LoadInjectors type-checks the package in dir and parses the named injectors
// out of Wire's output. It assumes that output already exists, and that
// names holds at least one injector: a load asking for none yields a
// package with none.
//
// Syntax and type information are loaded for the package in dir alone. Its
// dependencies contribute types through export data.
func (g *Generator) LoadInjectors(ctx context.Context, dir string, names []string) (*LoadedPackage, error) {
	return g.load(ctx, dir, names)
}

// load type-checks the package in dir and parses the injectors that names
// holds out of Wire's output.
func (g *Generator) load(ctx context.Context, dir string, names []string) (*LoadedPackage, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Fset:       fset,
		BuildFlags: g.parseBuildFlags,
		ParseFile:  g.parseWithoutLineDirectives,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("yama: loading package in %s: %w", dir, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("yama: no package found in %s", dir)
	}

	pkg := pkgs[0]
	if loadErr := packageErrors(pkg); loadErr != nil {
		return nil, fmt.Errorf("yama: loading package in %s: %w", dir, loadErr)
	}

	name := g.wireGenName

	wireGen := findWireGen(pkg, fset, name)
	if wireGen == nil {
		return nil, fmt.Errorf("yama: %s has no %s", dir, name)
	}

	parsed, err := ParseInjectors(fset, wireGen, names)
	if err != nil {
		return nil, err
	}

	return &LoadedPackage{ParsedFile: parsed, Package: pkg}, nil
}

// parseWithoutLineDirectives parses one file of the package under load. In
// g.wireGenName, parseWithoutLineDirectives blanks every line directive
// first, so a position that Yama reports out of that file names the file
// itself. Every other file parses unchanged.
//
// A nil src means the loader read nothing, and the file is read here.
func (g *Generator) parseWithoutLineDirectives(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if filepath.Base(filename) != g.wireGenName {
		return parser.ParseFile(fset, filename, src, parser.AllErrors|parser.ParseComments)
	}

	if src == nil {
		content, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		src = content
	}

	clean := dropLineDirectives(src)

	return parser.ParseFile(fset, filename, clean, parser.AllErrors|parser.ParseComments)
}

// findWireGen returns the package's Wire output: the file named name, which
// runWire's invocation produces. It returns nil if the package has none. A
// directory holds at most one such file.
func findWireGen(pkg *packages.Package, fset *token.FileSet, name string) *ast.File {
	for _, file := range pkg.Syntax {
		if filepath.Base(fset.Position(file.Pos()).Filename) == name {
			return file
		}
	}

	return nil
}
