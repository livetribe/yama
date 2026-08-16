// Package wire derives a Google Wire injector from a lifecycle stub, and it
// runs Google Wire over a set of directories.
//
// Derive takes data and returns bytes, so it needs no Go toolchain. Run is the
// one place in the tree that starts a subprocess.
package wire

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"l7e.io/yama/v2/internal/generator/sketch/source"
)

const (
	// Tag guards the derived injector file. Only Google Wire's own load sets
	// it, so every later load ignores that file.
	Tag = "wireinject"

	// DerivedFileName is the file that Derive's bytes go into.
	DerivedFileName = "yama_wireinject.go"

	// PlaceholderFileName is the file that DerivePlaceholder's bytes go into.
	PlaceholderFileName = "yama_placeholder.go"

	// derivedPrefix starts the name of a derived injector, so the injector
	// never collides with the stub it comes from.
	derivedPrefix = "yama_"

	// BaseOutputName is the file that Google Wire writes.
	BaseOutputName = "wire_gen.go"

	// PackagePath is Google Wire itself. A stub imports it for wire.Build, and
	// the lifecycle file that Yama writes never uses it.
	PackagePath = source.WirePackagePath
)

// fileMode is the mode that wire gives the derived file it writes.
const fileMode = 0o600

// linePrefix begins a Go line directive. A directive moves every position
// after it onto the file and line that it names.
const linePrefix = "//line "

// The markers of Google Wire's own diagnostic. wroteMarker introduces the path
// in the line that reports an output file. injectMarker introduces the injector
// name in the line that reports one injector.
const (
	wroteMarker  = ": wrote "
	injectMarker = "inject "
)

// placeholderPanic is the body of a lifecycle placeholder. A placeholder keeps
// the package type-checking while its committed file is set aside.
const placeholderPanic = "yama: %s is not generated"

// DerivedName returns the injector that Yama derives from the named stub.
func DerivedName(stub string) string {
	return derivedPrefix + stub
}

// OutputName returns the file that Google Wire writes for the given
// output-file prefix.
func OutputName(prefix string) string {
	return prefix + BaseOutputName
}

// Derive assembles the derived injector file for one target package. The file
// holds one injector for each stub, and Google Wire's own load is the one load
// that reads it.
//
// Derive panics when go/format rejects what it assembled. Every such failure is
// a bug in Derive itself.
func Derive(pkg *source.PackageInfo) []byte {
	body := assemble(pkg, writeInjector)

	return file(pkg, Tag, body, DerivedFileName)
}

// DerivePlaceholder assembles the placeholder file for one target package. The
// file holds one declaration of each stub's own name, and it stands in for the
// lifecycle file while that file is set aside.
//
// The placeholder file carries the build constraint that the lifecycle file
// carries. Google Wire's load reads it, and so does the load that reads Google
// Wire's output. The two loads set different tags and each one needs the name.
//
// DerivePlaceholder panics when go/format rejects what it assembled. Every such
// failure is a bug in DerivePlaceholder itself.
func DerivePlaceholder(pkg *source.PackageInfo) []byte {
	body := assemble(pkg, writePlaceholder)

	return file(pkg, "!"+source.Tag, body, PlaceholderFileName)
}

// assemble writes one declaration for each stub, with write.
func assemble(pkg *source.PackageInfo, write func(*bytes.Buffer, *source.StubInfo)) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "package %s\n", pkg.Name)

	for i := range pkg.Stubs {
		write(&buf, &pkg.Stubs[i])
	}

	return buf.Bytes()
}

// file puts the build constraint, the package clause, the import block, and the
// body together. It keeps the imports that the body refers to. Go rejects a
// file that imports a package it never uses, and each transient file uses only
// part of what the stub file imported.
//
// file panics when go/format rejects what it put together. Every such failure
// is a bug here.
func file(pkg *source.PackageInfo, constraint string, body []byte, name string) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "//go:build %s\n\n", constraint)
	fmt.Fprintf(&buf, "package %s\n", pkg.Name)

	used := source.Qualifiers(body)
	writeImports(&buf, needed(pkg, used))

	_, declarations, _ := bytes.Cut(body, []byte("\n"))
	buf.Write(declarations)

	src, err := format.Source(buf.Bytes())
	if err != nil {
		panic(fmt.Sprintf("wire: go/format rejected %s: %v\n%s", name, err, buf.String()))
	}

	return src
}

// needed returns the imports that the body refers to. Each one it returns
// carries an explicit name, so a path whose last element is not the package
// name still resolves.
func needed(pkg *source.PackageInfo, used map[string]bool) []source.Import {
	out := make([]source.Import, 0, len(pkg.Imports))

	for _, imp := range pkg.Imports {
		name := source.ImportName(imp, pkg.ImportNames)

		if used[name] {
			out = append(out, source.Import{Name: name, Path: imp.Path})
		}
	}

	return out
}

// writeDerivedFile puts the derived injector file in dir.
func writeDerivedFile(dir string, content []byte) error {
	return write(dir, DerivedFileName, content)
}

// writePlaceholderFile puts the placeholder file in dir.
func writePlaceholderFile(dir string, content []byte) error {
	return write(dir, PlaceholderFileName, content)
}

// write puts one of this package's transient files in dir.
func write(dir, name string, content []byte) error {
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, content, fileMode); err != nil {
		return fmt.Errorf("write %s in %s: %w", name, dir, err)
	}

	return nil
}

// removeDerivedFile takes the derived injector file and the placeholder file
// out of dir. A run removes them whatever else it settles, and a directory that
// holds neither is not an error.
func removeDerivedFile(dir string) error {
	injector := remove(dir, DerivedFileName)
	placeholder := remove(dir, PlaceholderFileName)

	return errors.Join(injector, placeholder)
}

// remove takes one of this package's transient files out of dir.
func remove(dir, name string) error {
	err := os.Remove(filepath.Join(dir, name))
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("remove %s in %s: %w", name, dir, err)
}

// Args are the flags that a run passes through to Google Wire. Each one mirrors
// a flag of `wire gen`.
type Args struct {
	HeaderFile string
	Tags       string
	Prefix     string
}

// TagList returns the build tags that the run set, one entry for each. Google
// Wire takes them as one space-separated value, and every load that Yama makes
// takes them one at a time.
func (a Args) TagList() []string {
	return strings.Fields(a.Tags)
}

// Flags returns the flag list that these arguments produce, in a fixed order.
func (a Args) Flags() []string {
	var flags []string

	for _, each := range []struct{ name, value string }{
		{"header_file", a.HeaderFile},
		{"tags", a.Tags},
		{"output_file_prefix", a.Prefix},
	} {
		if each.value != "" {
			flags = append(flags, "-"+each.name+"="+each.value)
		}
	}

	return flags
}

// Run invokes Google Wire once over patterns, from dir. It returns whatever
// Google Wire printed.
//
// Run reports a failure of either grain through its error. Google Wire rejecting
// an input and the tool failing to start are the same signal here: the run
// failed, and the diagnostic that Run returns states the cause.
func Run(ctx context.Context, dir string, patterns []string, args Args) (string, error) {
	if len(patterns) == 0 {
		return "", nil
	}

	argv := append([]string{"tool", "wire", "gen"}, args.Flags()...)
	argv = append(argv, patterns...)

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	logged := strings.TrimSpace(stderr.String() + stdout.String())

	if err == nil {
		return logged, nil
	}

	return logged, fmt.Errorf("google wire failed in %s: %w", dir, err)
}

// writeImports writes the import block of the derived file.
func writeImports(buf *bytes.Buffer, imports []source.Import) {
	if len(imports) == 0 {
		return
	}

	buf.WriteString("\nimport (\n")

	for _, imp := range imports {
		if imp.Name != "" {
			fmt.Fprintf(buf, "\t%s %q\n", imp.Name, imp.Path)

			continue
		}

		fmt.Fprintf(buf, "\t%q\n", imp.Path)
	}

	buf.WriteString(")\n")
}

// writeInjector writes the injector that Google Wire reads. The injector takes
// the stub's parameters without its trailing option parameter, and it keeps the
// stub's own provider list.
//
// The injector returns the stub's first result, a cleanup, and an error. Google
// Wire accepts no other result list, and it accepts a cleanup result whether or
// not a provider contributes one.
func writeInjector(buf *bytes.Buffer, stub *source.StubInfo) {
	writeLine(buf, stub)

	params := fieldList(stub.GraphParams())
	result := stub.ResultType()
	providers := strings.Join(stub.Providers, ", ")

	fmt.Fprintf(buf, "\nfunc %s(%s) (%s, func(), error) {\n", DerivedName(stub.Name), params, result)
	fmt.Fprintf(buf, "\tpanic(wire.Build(%s))\n}\n", providers)
}

// writePlaceholder writes a declaration of the stub's own name, so the package
// still type-checks while its committed file is set aside.
func writePlaceholder(buf *bytes.Buffer, stub *source.StubInfo) {
	writeLine(buf, stub)

	params := fieldList(stub.Params)
	results := fieldList(stub.Results)
	message := fmt.Sprintf(placeholderPanic, stub.Name)

	fmt.Fprintf(buf, "\nfunc %s(%s) (%s) {\n", stub.Name, params, results)
	fmt.Fprintf(buf, "\tpanic(%q)\n}\n", message)
}

// writeLine writes the line directive that puts the declaration after it at the
// stub's own position. Google Wire loads the file through go/packages, which
// applies the directive, so Google Wire reports the file that a person wrote.
//
// writeLine writes nothing for a stub with no position.
func writeLine(buf *bytes.Buffer, stub *source.StubInfo) {
	if stub.File == "" {
		return
	}

	fmt.Fprintf(buf, "\n%s%s:%d:%d\n", linePrefix, stub.File, stub.Line, stub.Column)
}

// DropLineDirectives blanks every line directive in src.
//
// Google Wire copies the directive that Derive wrote into its own output. A
// position read from that output would name the stub file, at a line the stub
// file does not hold: Google Wire writes one statement for each component, and
// the declaration the directive points into holds three lines.
//
// DropLineDirectives reads the scanned tokens, so a line inside a string
// literal keeps the characters it has. Go applies a directive that starts at
// column 1 alone, and a comment that carries the prefix anywhere else is
// ordinary text.
//
// Each directive gives way to a comment of its own width, so every offset and
// every line number in src keeps the value it had.
func DropLineDirectives(src []byte) []byte {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var sc scanner.Scanner
	sc.Init(file, src, nil, scanner.ScanComments)

	out := bytes.Clone(src)

	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}

		if tok != token.COMMENT || !strings.HasPrefix(lit, linePrefix) {
			continue
		}

		offset := file.Offset(pos)
		if offset > 0 && out[offset-1] != '\n' {
			continue
		}

		blank(out[offset : offset+len(lit)])
	}

	return out
}

// blank turns one directive into a comment of the same width.
func blank(directive []byte) {
	for i := range directive {
		directive[i] = ' '
	}

	copy(directive, "//")
}

// Diagnostics returns what a run prints from what Google Wire logged.
//
// It leaves out the line that names a file Google Wire wrote. That file is
// transient, and a run takes it back out of the package, so the line states
// something that is no longer true.
//
// It takes Yama's own prefix off the injector name that Google Wire names, so
// the diagnostic names the stub that a person wrote. Every other text on the
// line keeps the characters it has, a path with the same prefix among it.
func Diagnostics(logged string) string {
	var kept []string

	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, wroteMarker) {
			continue
		}

		kept = append(kept, injectorName(line))
	}

	joined := strings.Join(kept, "\n")

	return strings.TrimSpace(joined)
}

// injectorName takes Yama's own prefix off the injector name that one line
// names. A line that names no injector keeps the characters it has.
func injectorName(line string) string {
	marker := injectMarker + derivedPrefix

	head, tail, found := strings.Cut(line, marker)
	if !found {
		return line
	}

	return head + injectMarker + tail
}

// fieldList prints one parameter or result list.
func fieldList(fields []source.Field) string {
	printed := make([]string, 0, len(fields))

	for _, f := range fields {
		if f.Name == "" {
			printed = append(printed, f.Type)

			continue
		}

		printed = append(printed, f.Name+" "+f.Type)
	}

	return strings.Join(printed, ", ")
}
