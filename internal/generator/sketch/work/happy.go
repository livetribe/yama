package work

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/emit"
	"l7e.io/yama/v2/internal/generator/sketch/graph"
	"l7e.io/yama/v2/internal/generator/sketch/source"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// defaultOptsName is what a constructor forwards through when the stub bound
// no name to its option parameter.
const defaultOptsName = "opts"

type Happy struct {
	path          string
	custodian     *custody.Custodian
	intermediates *wire.IntermediateYamaFiles
	info          *source.PackageInfo

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
	info *source.PackageInfo,
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

// Prepare puts both generated files out of Google Wire's way, and it writes the
// intermediate files that Google Wire's load reads.
//
// A package that fails a later step settles through Complete, which puts back
// every name this run moved.
func (h *Happy) Prepare(_ context.Context) State {
	if err := h.custodian.SetAside(); err != nil {
		return h.prepareFailed(err)
	}

	if err := h.intermediates.Prepare(); err != nil {
		return h.prepareFailed(err)
	}

	return h
}

// Generate reads Google Wire's output and writes the lifecycle file.
//
// Generate tests the directory for Google Wire's output. Google Wire writes
// nothing for a package that it rejected. A directory with no output settles
// as a NoWireGen.
func (h *Happy) Generate(_ context.Context) State {
	name := h.custodian.WireOutputName()
	output := filepath.Join(h.path, name)

	src, err := os.ReadFile(output)
	if os.IsNotExist(err) {
		return &NoWireGen{custodian: h.custodian, intermediates: h.intermediates}
	}

	if err != nil {
		return h.failed(err)
	}

	h.info.AddImportsFrom(src)

	injectors, err := graph.Parse(wire.DropLineDirectives(src), h.derivedNames())
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
func (h *Happy) Complete(_ context.Context) error {
	return settle(h.custodian, h.intermediates, nil)
}

// report states the lifecycle file that Generate wrote. It names the package by
// the import path that package declares, and the file by its absolute path.
//
// The line takes the shape Google Wire takes for its own output, on the stream
// Google Wire writes its own to.
func (h *Happy) report(file string) {
	fmt.Fprintf(h.progress, "yama: %s: wrote %s\n", h.info.PkgPath, file)
}

// prepareFailed returns the state that a failed Prepare settles as.
func (h *Happy) prepareFailed(err error) State {
	return &PrepareFailed{custodian: h.custodian, intermediates: h.intermediates, err: err}
}

// derivedNames returns the injector that Yama derived for each stub, which is
// what Google Wire's output declares.
func (h *Happy) derivedNames() []string {
	names := make([]string, 0, len(h.info.Stubs))

	for i := range h.info.Stubs {
		names = append(names, wire.DerivedName(h.info.Stubs[i].Name))
	}

	return names
}

// lifecycleFile assembles what emit renders. It pairs each stub with the
// injector that Yama derived from it, and it turns that injector's components
// into lifecycle levels.
func (h *Happy) lifecycleFile(injectors []graph.Injector, scope []string) emit.Package {
	byName := make(map[string]graph.Injector, len(injectors))
	for _, inj := range injectors {
		byName[inj.Name] = inj
	}

	imports := h.imports()
	names := nameFile(imports, injectors, scope, h.info.Stubs)

	file := emit.Package{Name: h.info.Name, Imports: imports, Yama: names.yama, Rt: names.rt}

	for i := range h.info.Stubs {
		stub := &h.info.Stubs[i]

		inj, ok := byName[wire.DerivedName(stub.Name)]
		if !ok {
			continue
		}

		levels := graph.Levels(inj.Components)

		file.Constructors = append(file.Constructors, emit.Constructor{
			Doc:        stub.Doc,
			Signature:  signature(stub, names.opts[i], names.yama),
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

// imports carries onto the lifecycle file every import that the constructor
// could refer to: the stub file's, which the signature names, and Google Wire's
// own, which the construction names. Each one carries the name that its package
// declares, and emit leaves out the ones that the rendered file never refers to.
//
// imports leaves Google Wire out: a stub imports it for wire.Build, and the
// lifecycle file calls nothing from it.
//
// One path under two names reaches the file twice. The signature may name a
// package by the alias a stub gave it, and the construction by the name that
// Google Wire used, and Go takes one path under two names.
func (h *Happy) imports() []emit.Import {
	stated := make([]source.Import, 0, len(h.info.Imports)+len(h.info.OutputImports))
	stated = append(stated, h.info.Imports...)
	stated = append(stated, h.info.OutputImports...)

	out := make([]emit.Import, 0, len(stated))
	seen := make(map[emit.Import]bool, len(stated))

	for _, imp := range stated {
		if imp.Path == wire.PackagePath {
			continue
		}

		name := source.ImportName(imp, h.info.ImportNames)

		entry := emit.Import{Name: name, Path: imp.Path}
		if seen[entry] {
			continue
		}

		seen[entry] = true
		out = append(out, entry)
	}

	return out
}

// optsName names the parameter that the constructor forwards its options
// through. It is empty for a stub that declared none.
func optsName(stub *source.StubInfo) string {
	if !stub.HasOpts {
		return ""
	}

	last := stub.Params[len(stub.Params)-1]
	if last.Name == "" || last.Name == "_" {
		return defaultOptsName
	}

	return last.Name
}

// signature prints the constructor that the lifecycle file declares. It is the
// stub's own signature.
func signature(stub *source.StubInfo, opts, yama string) string {
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

		typ := requalify(p.Type, stub.Yama, yama)

		if name == "" {
			params = append(params, typ)

			continue
		}

		params = append(params, name+" "+typ)
	}

	results := make([]string, 0, len(stub.Results))
	for _, r := range stub.Results {
		results = append(results, requalify(r.Type, stub.Yama, yama))
	}

	return fmt.Sprintf("func %s(%s) (%s)", stub.Name, strings.Join(params, ", "), strings.Join(results, ", "))
}
