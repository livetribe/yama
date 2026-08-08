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

// Command yama runs Google Wire and yama's generator over one or more target
// packages to produce lifecycle_gen.go, the one committed generated file. Wire's
// wire_gen.go is a transient intermediate the command removes when it is done.
//
// Its flags and package-pattern argument mirror `wire gen` exactly, so a
// project's existing go:generate directive for Google Wire carries over
// unchanged when it names yama instead.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"l7e.io/yama/v2/internal/generator"
)

// The exit codes are `wire gen`'s own, measured against it: 2 for arguments it
// cannot parse, 1 for a run that failed.
const (
	usageExitCode   = 2
	failureExitCode = 1
)

func main() {
	ctx := context.Background()

	err := run(ctx, os.Stderr, os.Args[1:])
	if err == nil {
		return
	}

	// The flag package writes the diagnostic and the usage text as it parses. A
	// usage error is already on the stream; printing it here would repeat it.
	var usage *usageError
	if errors.As(err, &usage) {
		os.Exit(usageExitCode)
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(failureExitCode)
}

// run generates for the packages args names, writing progress and diagnostics
// to stderr.
func run(ctx context.Context, stderr io.Writer, args []string) error {
	parsed, err := parseArgs(stderr, args)
	if err != nil {
		return err
	}

	g := generator.NewGenerator(parsed.opts)

	files, err := g.EmitAll(ctx, ".", parsed.patterns)
	if err != nil {
		return err
	}

	return writeFiles(stderr, files)
}

// writeFiles commits each emitted lifecycle file and reports one progress line
// for each one written.
//
// The line names Yama's own artifact in the shape Wire uses for its own,
// `yama: <import path>: wrote <path>`, on the stream Wire writes its own to.
//
// A write that fails does not stop the writes that follow it. Every failure
// reaches the caller together.
func writeFiles(progress io.Writer, files []*generator.LifecycleFile) error {
	var errs []error
	for _, file := range files {
		path, err := file.Write()
		if err != nil {
			errs = append(errs, err)

			continue
		}

		fmt.Fprintf(progress, "yama: %s: wrote %s\n", file.PkgPath, path)
	}

	return errors.Join(errs...)
}

// usageError reports arguments the command cannot parse. It carries what the
// flag package reported, which the flag package has also already written to the
// command's error stream together with the usage text.
type usageError struct {
	Err error
}

func (e *usageError) Error() string {
	return e.Err.Error()
}

func (e *usageError) Unwrap() error {
	return e.Err
}

// parsedArgs is the command's flags and positional package patterns, kept
// separate from execution so flag parsing is testable without touching the
// filesystem or running Wire.
type parsedArgs struct {
	opts     generator.Options
	patterns []string
}

// parseArgs parses args into flags and package patterns, writing what it cannot
// parse to out. Flag names and defaults match `wire gen` exactly (see `go tool
// wire help gen`); an unrecognized flag is reported by the flag package in the
// same shape wire itself would report it, and arrives back as a *usageError.
func parseArgs(out io.Writer, args []string) (parsedArgs, error) {
	fs := flag.NewFlagSet("yama", flag.ContinueOnError)
	fs.SetOutput(out)

	var parsed parsedArgs
	fs.StringVar(&parsed.opts.HeaderFile, "header_file", "",
		"path to file to insert as a header in the generated output")
	fs.StringVar(&parsed.opts.OutputFilePrefix, "output_file_prefix", "",
		"string to prepend to output file names")
	fs.StringVar(&parsed.opts.Tags, "tags", "", "append build tags to the default yama build")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: yama [flags] [packages]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "  Given one or more packages, yama generates lifecycle_gen.go for each.")
		fmt.Fprintln(fs.Output(), "  If no packages are listed, it defaults to \".\".")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return parsedArgs{}, &usageError{Err: err}
	}
	parsed.patterns = fs.Args()

	return parsed, nil
}
