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
	"errors"
	"fmt"

	"golang.org/x/tools/go/packages"
)

// wireGenBaseName is the file name Google Wire writes, before any output-file
// prefix is applied.
const wireGenBaseName = "wire_gen.go"

// wireInjectTag gates an injector definition. Google Wire loads under it to see
// injectors, and marks its own generated file `!wireinject` so the two never
// compile together.
const wireInjectTag = "wireinject"

// Options configures a generation run. Every field mirrors one of `wire gen`'s
// own flags.
type Options struct {
	// OutputFilePrefix is handed to Google Wire's -output_file_prefix flag. Wire
	// prepends it to the output file name verbatim, inserting no separator, so a
	// prefix of "foo_" yields foo_wire_gen.go.
	OutputFilePrefix string

	// HeaderFile is handed to Google Wire's -header_file flag: a file with
	// content that is inserted at the top of wire_gen.go. Since wire_gen.go is
	// transient, the header itself is never seen by anyone. A malformed header
	// surfaces as a diagnostic from Wire, not from Yama.
	HeaderFile string

	// Tags is handed to Google Wire's -tags flag. It also carries into Yama's own
	// package load: a provider Wire was told to include because of a build tag
	// is one Yama's own type-checking must see too, since Wire's generated file
	// references it by name and the symbol only exists in the package under the
	// same tags.
	Tags string
}

// wireGenName is the file that Google Wire writes under these options. It is
// built by Wire's own rule, so the file that Yama looks for, preserves, and
// removes is always exactly the file that Wire wrote.
func (o Options) wireGenName() string {
	return o.OutputFilePrefix + wireGenBaseName
}

// wireArgs is the `wire gen` flag list that these options produce, built as
// its own method so it can be asserted on directly, without a subprocess.
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

// stubBuildFlags are the build flags for loading a package's lifecycle stubs.
// A stub only exists under the yamainject tag, and Yama's own emitted file is
// `//go:build !yamainject`, so one load both sees the stubs and excludes the
// constructors emitted for them.
func (o Options) stubBuildFlags() []string {
	tags := yamaInjectTag
	if o.Tags != "" {
		tags += " " + o.Tags
	}

	return []string{"-tags=" + tags}
}

// parseBuildFlags are the build flags for loading Wire's generated output. They
// deliberately omit the wireinject tag: the generated file is
// `//go:build !wireinject`, so it is invisible under the tag that makes the
// injector that it was generated from visible.
func (o Options) parseBuildFlags() []string {
	if o.Tags == "" {
		return nil
	}

	return []string{"-tags=" + o.Tags}
}

// ToolError reports that Google Wire ran, failed, and logged a diagnostic of its
// own. The diagnostic names the fault: a problem in the injector definitions, or
// a problem that Wire found elsewhere. It carries Wire's output so the underlying
// failure is visible, and is distinct from a ToolchainError.
type ToolError struct {
	Dir    string
	Stderr string
	Err    error
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("yama: wire generation failed in %s: %s", e.Dir, e.Stderr)
}

func (e *ToolError) Unwrap() error {
	return e.Err
}

// ToolchainError reports that the Google Wire tool could not be run at all.
// Either it is not installed as a Go tool, or the go command itself failed
// to launch it. It is distinct from a ToolError, which means Wire ran and
// found a problem.
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

// LoadedPackage is a parsed wire_gen.go together with its type-checked package.
// The parsed injectors reference AST nodes owned by Package.Syntax, so a
// component's concrete type can be resolved through Package.TypesInfo without
// re-parsing.
type LoadedPackage struct {
	*ParsedFile
	Package *packages.Package
}

// packageErrors joins what a load reported against pkg. It returns nil when the
// load reported nothing.
//
// A package that does not type-check yields values with no resolved type. A
// caller that reads one reports a Yama defect for an error in the application's
// own source.
func packageErrors(pkg *packages.Package) error {
	if len(pkg.Errors) == 0 {
		return nil
	}

	errs := make([]error, len(pkg.Errors))
	for i, e := range pkg.Errors {
		errs[i] = e
	}

	return errors.Join(errs...)
}
