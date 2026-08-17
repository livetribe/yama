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

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/emit"
	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/pkg"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

type Happy struct {
	path          string
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	info          *pkg.Info

	header []byte
	tags   []string

	// progress is the stream that Generate reports the file it wrote on.
	progress io.Writer
}

var _ State = (*Happy)(nil)

// NewHappy returns the item for a target package that declares at least one
// lifecycle stub. info holds the facts that the run read from the package.
func NewHappy(
	path string,
	custodian *custody.Custodian,
	intermediates *wire.IntermediateYamaFiles,
	header []byte,
	tags []string,
	info *pkg.Info,
	progress io.Writer,
) *Happy {
	return &Happy{
		path:          path,
		custodian:     custodian,
		intermediates: intermediates,
		info:          info,
		header:        header,
		tags:          tags,
		progress:      progress,
	}
}

func (h *Happy) PackagePath() (path string, ok bool) {
	return h.path, true
}

// Prepare writes the intermediate files that Google Wire's load reads, and it
// puts both generated files out of Google Wire's way.
//
// The intermediate files go in first. A package that fails to take them keeps
// the lifecycle file that it committed, and every package that imports it still
// declares what that file declares.
//
// A package that fails a later step settles through Complete, which puts back
// every name this run moved.
func (h *Happy) Prepare() State {
	if err := h.intermediates.Prepare(); err != nil {
		return h.prepareFailed(err)
	}

	if err := h.custodian.SetAside(); err != nil {
		return h.prepareFailed(err)
	}

	return h
}

// Generate reads Google Wire's output and writes the lifecycle file.
//
// Generate tests the directory for Google Wire's output. Google Wire writes
// nothing for a package that it rejected. A directory with no output settles
// as a NoWireGen.
func (h *Happy) Generate() State {
	name := h.custodian.WireOutputName()
	output := filepath.Join(h.path, name)

	src, err := os.ReadFile(output)
	if os.IsNotExist(err) {
		return &NoWireGen{custodian: h.custodian, intermediates: h.intermediates}
	}

	if err != nil {
		return h.failed(err)
	}

	if err = h.info.TakeOutputImportsFrom(src); err != nil {
		return h.failed(err)
	}

	body := wire.DropLineDirectives(src)
	derived := wire.DerivedNames(h.info)

	injectors, err := graph.Parse(body, derived)
	if err != nil {
		return h.failed(err)
	}

	injectors, scope, err := graph.Detect(h.path, h.tags, injectors)
	if err != nil {
		return h.failed(err)
	}

	content := emit.Render(h.lifecycleFile(injectors, scope), h.header)

	written, err := emit.Write(h.path, h.custodian.LifecycleFileName(), content)
	if err != nil {
		return h.failed(err)
	}

	h.report(written)

	if err := h.clearTransients(output); err != nil {
		return h.failed(err)
	}

	return h
}

// Complete settles the package's files and removes the intermediate files.
func (h *Happy) Complete() error {
	return settle(h.custodian, h.intermediates, nil)
}

// report states the lifecycle file that Generate wrote. It names the package by
// the import path that package declares, and the file by its absolute path.
//
// The line takes the shape Google Wire takes for its own output, on the stream
// Google Wire writes its own to.
func (h *Happy) report(file string) {
	fmt.Fprintf(h.progress, "yama: %s: wrote %s\n", h.info.PkgPath(), file)
}

// prepareFailed returns the state that a failed Prepare settles as.
func (h *Happy) prepareFailed(err error) State {
	return &PrepareFailed{custodian: h.custodian, intermediates: h.intermediates, err: err}
}

// lifecycleFile assembles what emit renders. It pairs each stub with the
// injector that Yama derived from it, and it turns that injector's components
// into lifecycle levels.
func (h *Happy) lifecycleFile(injectors []graph.Injector, scope []string) emit.Package {
	byName := make(map[string]graph.Injector, len(injectors))
	for _, inj := range injectors {
		byName[inj.Name] = inj
	}

	imports := h.info.LifecycleImports()
	names := nameFile(imports, injectors, scope, h.info.Stubs())
	rtFrom := h.info.ImportedAs(rtPath)

	file := emit.Package{Name: h.info.Name(), Imports: imports, Yama: names.yama, Rt: names.rt}

	for i := range h.info.Stubs() {
		stub := &h.info.Stubs()[i]

		inj, ok := byName[wire.DerivedName(stub.Name)]
		if !ok {
			continue
		}

		levels := graph.Levels(inj.Components)

		file.Constructors = append(file.Constructors, emit.Constructor{
			Doc:        stub.Doc,
			Signature:  signature(stub, names.opts[i], &names, rtFrom),
			Statements: inj.Statements,
			Result:     inj.Result,
			Levels:     members(levels),
			Opts:       names.opts[i],
		})
	}

	return file
}

// clearTransients takes Google Wire's output and the intermediate files out of
// the directory. Generate removes them only after it wrote the lifecycle file,
// so a package that failed to render keeps Wire's output for the rest of the
// loop.
func (h *Happy) clearTransients(output string) error {
	if err := os.Remove(output); err != nil {
		return fmt.Errorf("remove %s: %w", filepath.Base(output), err)
	}

	return h.intermediates.CleanUp()
}

// failed returns the state that a failed Generate settles as.
func (h *Happy) failed(err error) State {
	return &GenerateFailed{custodian: h.custodian, intermediates: h.intermediates, err: err}
}

// members carries each level onto the lifecycle file.
func members(levels [][]graph.Member) [][]emit.Member {
	out := make([][]emit.Member, len(levels))

	for i, level := range levels {
		for _, m := range level {
			out[i] = append(out[i], emit.Member{
				Name:    m.Name,
				Cleanup: m.Cleanup,
				Capable: m.Capabilities != graph.None,
			})
		}
	}

	return out
}

// signature prints the constructor that the lifecycle file declares. It is the
// stub's own signature, under the names that the lifecycle file gives Yama's
// own two packages.
//
// rtFrom is the name that the stub's file refers to Yama's runtime package by.
// The lifecycle file drops that import and states one of its own, so every type
// that names the runtime package takes the new name.
func signature(stub *pkg.Stub, opts string, names *naming, rtFrom string) string {
	params := make([]string, 0, len(stub.Params))
	last := len(stub.Params) - 1

	for i, p := range stub.Params {
		name := p.Name

		// The constructor forwards its options, so the parameter that carries
		// them takes the name it is forwarded under. A stub may bind that
		// parameter to the blank identifier, which names nothing.
		if stub.HasOpts && i == last {
			name = opts
		}

		typ := renamePackages(p.Type, stub, names, rtFrom)

		if name == "" {
			params = append(params, typ)

			continue
		}

		params = append(params, name+" "+typ)
	}

	results := make([]string, 0, len(stub.Results))

	for _, r := range stub.Results {
		typ := renamePackages(r.Type, stub, names, rtFrom)
		results = append(results, typ)
	}

	return fmt.Sprintf("func %s(%s) (%s)", stub.Name, strings.Join(params, ", "), strings.Join(results, ", "))
}

// renamePackages returns one printed type under the names that the lifecycle
// file gives Yama's own two packages.
func renamePackages(typ string, stub *pkg.Stub, names *naming, rtFrom string) string {
	renamed := requalify(typ, stub.YamaName(), names.yama)

	return requalify(renamed, rtFrom, names.rt)
}
