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
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// wireGenBaseName is the file name Google Wire writes, before any output-file
// prefix is applied.
const wireGenBaseName = "wire_gen.go"

// wireInjectTag gates an injector definition. Google Wire loads under it to see
// injectors, and marks its own generated file `!wireinject` so the two never
// compile together.
const wireInjectTag = "wireinject"

// wirePrefix begins every line Google Wire logs, and so distinguishes Wire's own
// diagnostics from the go command's.
const wirePrefix = "wire:"

// wireWroteMarker introduces the path in Google Wire's per-output success line,
// `wire: <pkgPath>: wrote <path>`. wireDiagnostics drops the lines carrying it.
const wireWroteMarker = ": wrote "

// Options configures a generation run. Every field mirrors one of `wire gen`'s
// own flags, so a project's existing go:generate invocation of Google Wire
// carries over unchanged when it is replaced with Yama's.
type Options struct {
	// OutputFilePrefix is handed to Google Wire's -output_file_prefix flag. Wire
	// prepends it to the output file name verbatim, inserting no separator, so a
	// prefix of "foo_" yields foo_wire_gen.go.
	OutputFilePrefix string

	// HeaderFile is handed to Google Wire's -header_file flag: a file whose
	// content is inserted at the top of wire_gen.go. Since wire_gen.go is
	// transient, the header itself is never seen by anyone; the flag is threaded
	// through for parity with wire gen and because a malformed header is a
	// diagnostic Wire itself should report.
	HeaderFile string

	// Tags is handed to Google Wire's -tags flag. It also carries into Yama's own
	// package load: a provider Wire was told to include because of a build tag
	// is one Yama's own type-checking must see too, since Wire's generated file
	// references it by name and the symbol only exists in the package under the
	// same tags.
	Tags string
}

// wireGenName is the file Google Wire writes under these options. It is built by
// Wire's own rule, so the file Yama looks for, preserves, and removes is always
// exactly the file Wire wrote.
func (o Options) wireGenName() string {
	return o.OutputFilePrefix + wireGenBaseName
}

// wireArgs is the `wire gen` flag list these options produce. Built as its own
// method so it can be asserted on directly, without a subprocess.
func (o Options) wireArgs() []string {
	var args []string
	if o.OutputFilePrefix != "" {
		args = append(args, "-output_file_prefix="+o.OutputFilePrefix)
	}
	if o.HeaderFile != "" {
		args = append(args, "-header_file="+o.HeaderFile)
	}
	if o.Tags != "" {
		args = append(args, "-tags="+o.Tags)
	}

	return args
}

// discoveryBuildFlags are the build flags for finding the packages Wire will
// generate for. They mirror Google Wire's own loader byte for byte: Wire always
// loads under the wireinject tag, appending the caller's tags to it, because an
// injector only exists under that tag. Yama must resolve the same package set
// Wire will, so it loads the same way.
func (o Options) discoveryBuildFlags() []string {
	tags := wireInjectTag
	if o.Tags != "" {
		tags += " " + o.Tags
	}

	return []string{"-tags=" + tags}
}

// parseBuildFlags are the build flags for loading Wire's generated output. They
// deliberately omit the wireinject tag that discoveryBuildFlags sets: the
// generated file is `//go:build !wireinject`, so it is invisible under the tag
// that makes its injector visible.
func (o Options) parseBuildFlags() []string {
	if o.Tags == "" {
		return nil
	}

	return []string{"-tags=" + o.Tags}
}

// GenerateError reports that Google Wire ran but rejected the package's Wire
// input — a missing provider, a cycle, or another problem in the injector
// definitions. It carries Wire's own diagnostic so the underlying failure is
// visible, and is distinct from a ToolchainError.
type GenerateError struct {
	Dir    string
	Stderr string
	Err    error
}

func (e *GenerateError) Error() string {
	return fmt.Sprintf("yama: wire generation failed in %s: %s", e.Dir, e.Stderr)
}

func (e *GenerateError) Unwrap() error {
	return e.Err
}

// ToolchainError reports that the Google Wire tool could not be run at all — it
// is not installed as a Go tool, or the go command itself failed to launch it.
// It is distinct from a GenerateError, which means Wire ran and found a problem.
type ToolchainError struct {
	Stderr string
	Err    error
}

func (e *ToolchainError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("yama: could not run the wire tool: %v: %s", e.Err, e.Stderr)
	}

	return fmt.Sprintf("yama: could not run the wire tool: %v", e.Err)
}

func (e *ToolchainError) Unwrap() error {
	return e.Err
}

// ErrNoInjectors reports that a package has no Wire output for Load to find —
// either because Wire ran and found no injector function to generate (confirmed
// against a real invocation: Wire treats an injector-free package as a silent
// no-op, exit 0, no file written) or because Wire has not been run there. It is
// not a failure; GenerateAll uses it to skip a package in a multi-package sweep
// the same way Wire itself silently skips one.
var ErrNoInjectors = errors.New("yama: package has no Wire injector")

// LoadedPackage is a parsed wire_gen.go together with its type-checked package.
// The parsed injectors reference AST nodes owned by Package.Syntax, so a
// component's concrete type can be resolved through Package.TypesInfo without
// re-parsing.
type LoadedPackage struct {
	*ParsedFile
	Package *packages.Package
}

// runWire runs `go tool wire gen` once over patterns, from wd, and returns
// everything Wire logged.
//
// One invocation from the caller's own directory is how Google Wire runs itself,
// and matching that is what keeps behavior identical: a relative -header_file
// resolves against wd exactly as Wire resolves it, and Wire's diagnostics arrive
// in Wire's own order rather than reordered by a per-package loop.
//
// It overwrites any Wire output under those patterns, because that is what
// invoking Wire does, and it is deliberately unexported: GenerateAll is the only
// way in, so the destructive step is always wrapped in the scopes that preserve
// the caller's files.
//
// A Wire input problem surfaces as a *GenerateError naming Wire's own diagnostic;
// an inability to launch the tool surfaces as a *ToolchainError. The two are
// distinct so a build failure points at the right cause.
func (g *Generator) runWire(ctx context.Context, wd string, patterns []string) error {
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

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && wireReported(logged) {
		return &GenerateError{Dir: wd, Stderr: wireDiagnostics(logged), Err: err}
	}

	return &ToolchainError{Stderr: wireDiagnostics(logged), Err: err}
}

// wireDiagnostics is Wire's output with its per-output success lines removed.
//
// Wire reports `wrote <path>` for each file it produced, but those files are
// transient here and gone by the time anything is printed. Passing the lines
// through would tell the caller Yama wrote files it had already deleted, so only
// the diagnostics that remain true are kept.
func wireDiagnostics(logged string) string {
	var kept []string
	for _, line := range strings.Split(logged, "\n") {
		if !strings.Contains(line, wireWroteMarker) {
			kept = append(kept, line)
		}
	}

	return strings.TrimSpace(strings.Join(kept, "\n"))
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

// Load type-checks the package in dir and parses Wire's output into the model. It
// assumes that output already exists; use Generate to run Wire first.
//
// Syntax and type information are loaded for the package in dir alone; its
// dependencies contribute types through export data.
func (g *Generator) Load(ctx context.Context, dir string) (*LoadedPackage, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Context:    ctx,
		Dir:        dir,
		Fset:       fset,
		BuildFlags: g.parseBuildFlags,
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
	name := g.wireGenName

	wireGen := findWireGen(pkg, fset, name)
	if wireGen == nil {
		return nil, fmt.Errorf("%w: %s has no %s", ErrNoInjectors, dir, name)
	}

	parsed, err := Parse(fset, wireGen)
	if err != nil {
		return nil, err
	}

	return &LoadedPackage{ParsedFile: parsed, Package: pkg}, nil
}

// Generate runs Google Wire for the single package in dir and parses the result.
//
// Wire's output is transient: it exists only while the graph is read, and dir is
// left as Generate found it. An output file already present is preserved and put
// back untouched — Generate removes only a file it generated itself — and the
// directory is restored even when generation or parsing fails.
//
// It is GenerateAll over the one package in dir, so both paths run Wire the same
// way; a relative HeaderFile therefore resolves against dir here.
func (g *Generator) Generate(ctx context.Context, dir string) (*LoadedPackage, error) {
	pkgs, err := g.GenerateAll(ctx, dir, []string{"."})
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%w: %s has no %s", ErrNoInjectors, dir, g.wireGenName)
	}

	return pkgs[0], nil
}

// findWireGen returns the package's Wire output — the file named name, which
// runWire's invocation produces — or nil if the package has none. A directory
// holds at most one such file.
func findWireGen(pkg *packages.Package, fset *token.FileSet, name string) *ast.File {
	for _, file := range pkg.Syntax {
		if filepath.Base(fset.Position(file.Pos()).Filename) == name {
			return file
		}
	}

	return nil
}
