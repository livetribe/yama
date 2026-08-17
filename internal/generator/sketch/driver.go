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

package sketch

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"

	"l7e.io/yama/v2/internal/generator/sketch/pkg"
	"l7e.io/yama/v2/internal/generator/sketch/resolve"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
	"l7e.io/yama/v2/internal/generator/sketch/work"
)

// headerCheckClause stands in for the package clause that follows a header in
// every file that a run writes.
const headerCheckClause = "package yama\n"

// A Driver runs one generation over a set of target packages. It holds no
// per-package state, it moves no file, and it composes no path.
type Driver struct {
	dir      string
	patterns []string
	args     wire.Args
	progress io.Writer
}

// NewDriver returns a Driver over patterns. Each one takes Go's own package
// pattern syntax, "./..." among it. dir is the directory that Google Wire runs
// from, and the directory that a pattern resolves against.
//
// A run writes what it did to os.Stderr. A test in this package assigns
// progress a buffer of its own to read that output back.
func NewDriver(dir string, patterns []string, args wire.Args) *Driver {
	return &Driver{dir: dir, patterns: patterns, args: args, progress: os.Stderr}
}

// Run creates one work item for each target package, and it puts every item
// through three phases in order. It invokes Google Wire once, between the first
// phase and the second.
//
// A run has two error channels, and both are bare signals. Google Wire's own
// failure travels on RunWire. A package's failure travels on its item, and it
// reaches Run through Complete.
func (d *Driver) Run(ctx context.Context) error {
	header, err := d.header()
	if err != nil {
		return err
	}

	tags := d.args.TagList()

	targets, err := resolve.Packages(ctx, d.dir, stubTags(tags), d.patterns)
	if err != nil {
		return err
	}

	items := work.CreateWorkItems(targets, d.args.Prefix, header, tags, d.progress)

	for i, item := range items {
		items[i] = item.Prepare(ctx)
	}

	wireErr := d.RunWire(ctx, items)

	for i, item := range items {
		items[i] = item.Generate(ctx)
	}

	errs := []error{wireErr}

	for _, item := range items {
		errs = append(errs, item.Complete(ctx))
	}

	return errors.Join(errs...)
}

// RunWire invokes Google Wire once over the directories that are still in the
// run. It prints Google Wire's diagnostic where the failure happened, and it
// converts no item.
//
// The error that RunWire returns carries no detail. Google Wire already stated
// the cause in the diagnostic that RunWire printed.
func (d *Driver) RunWire(ctx context.Context, items work.Items) error {
	paths := items.Paths()

	logged, err := wire.Run(ctx, d.dir, paths, d.args)

	diagnostic := wire.Diagnostics(logged)
	if diagnostic != "" {
		fmt.Fprintln(d.progress, diagnostic)
	}

	return err
}

// header reads the file that the run's -header_file names. It returns nil when
// the run named none. A relative name is relative to the directory that Google
// Wire runs from, which is what Google Wire itself does with that flag.
//
// header reports a file that no Go file can carry above its package clause.
func (d *Driver) header() ([]byte, error) {
	if d.args.HeaderFile == "" {
		return nil, nil
	}

	path := d.args.HeaderFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(d.dir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read header file %s: %w", d.args.HeaderFile, err)
	}

	if err := checkHeader(content); err != nil {
		return nil, fmt.Errorf("read header file %s: %w", d.args.HeaderFile, err)
	}

	return content, nil
}

// checkHeader reports a header that a Go file cannot carry. Every file that a
// run writes puts the header above its own package clause, so a header holds
// comments and blank lines alone.
func checkHeader(content []byte) error {
	src := string(content) + "\n\n" + headerCheckClause

	_, err := parser.ParseFile(token.NewFileSet(), "header.go", src, parser.PackageClauseOnly)
	if err != nil {
		return fmt.Errorf("it holds something that is not a comment: %w", err)
	}

	return nil
}

// stubTags returns the tags that resolution loads under. A package whose only
// file is a lifecycle stub holds no file without the stub tag, and a run must
// still reach it.
func stubTags(tags []string) []string {
	stub := make([]string, 0, len(tags)+1)
	stub = append(stub, pkg.Tag)

	return append(stub, tags...)
}
