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
	"flag"
	"fmt"
	"os"

	"l7e.io/yama/v2/internal/generator"
)

func main() {
	ctx := context.Background()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run drives generation from parsed args.
func run(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args)
	if err != nil {
		return err
	}

	g := generator.NewGenerator(parsed.opts)

	_, err = g.GenerateAll(ctx, ".", parsed.patterns)

	return err
}

// parsedArgs is the command's flags and positional package patterns, kept
// separate from execution so flag parsing is testable without touching the
// filesystem or running Wire.
type parsedArgs struct {
	opts     generator.Options
	patterns []string
}

// parseArgs parses args into flags and package patterns. Flag names and
// defaults match `wire gen` exactly (see `go tool wire help gen`); an
// unrecognized flag is reported by the flag package in the same shape wire
// itself would report it.
func parseArgs(args []string) (parsedArgs, error) {
	fs := flag.NewFlagSet("yama", flag.ContinueOnError)

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
		return parsedArgs{}, err
	}
	parsed.patterns = fs.Args()

	return parsed, nil
}
