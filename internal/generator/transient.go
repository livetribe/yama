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
	"io/fs"
	"os"
	"path/filepath"
)

// backupNamePrefix marks the file holding a preserved Wire output. It is prepended
// to whatever name Wire wrote, so the backup tracks the output-file prefix instead
// of assuming one: a foo_wire_gen.go is held at .yama.foo_wire_gen.go.
//
// The leading dot keeps the Go toolchain from reading the backup as a source file,
// which is what lets it sit beside the regenerated one without colliding.
const backupNamePrefix = ".yama."

// backupNameFor is where a Wire output named name is held while Yama regenerates.
func backupNameFor(name string) string {
	return backupNamePrefix + name
}

// wireGenScope makes Google Wire's output transient and generation
// non-destructive.
//
// Wire writes its output into the package directory and offers no way to redirect
// it, so a file of that name already there is moved aside for the life of the scope
// and put back by restore. Only a file that appeared while the scope was open, which
// is Yama's own, is ever removed.
//
// A directory that holds no file of that name gets no backup. A backup is taken
// only to hold a caller's file, so one already there is a caller's file an earlier
// run did not put back, and this scope's restore puts it back.
type wireGenScope struct {
	dir        string
	name       string
	backupName string
	path       string
	backup     string
}

// openWireGenScope prepares dir for a transient Wire output named name. It
// moves any existing file of that name aside.
//
// A backup already in dir holds a caller's file. The scope keeps it and removes
// any file at the name that it came from, which an earlier run generated and
// did not remove. That removed file would otherwise leave a stale copy in the
// load that Google Wire and Yama both run over the package.
//
// Otherwise, when dir holds no backup yet, a file of that name is moved onto
// the backup name, and a directory with no such file gets no backup at all.
func openWireGenScope(dir, name string) (*wireGenScope, error) {
	s := &wireGenScope{
		dir:        dir,
		name:       name,
		backupName: backupNameFor(name),
	}
	s.path = filepath.Join(dir, s.name)
	s.backup = filepath.Join(dir, s.backupName)

	_, backupErr := os.Lstat(s.backup)
	if backupErr == nil {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("yama: removing generated %s in %s: %w", s.name, dir, err)
		}

		return s, nil
	}
	if !errors.Is(backupErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("yama: inspecting %s in %s: %w", s.backupName, dir, backupErr)
	}

	if err := os.Rename(s.path, s.backup); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Nothing of that name to preserve.
			return s, nil
		}

		return nil, fmt.Errorf("yama: preserving existing %s in %s: %w", s.name, dir, err)
	}

	return s, nil
}

// putBackInterrupted moves back what an earlier run moved aside and did not put
// back, over every directory in dirs, for every transient file name.
//
// A directory that holds no backup is left as it is. Only a move back happens
// here, never a move aside, so a directory that this run goes on to skip is
// never left short of a file that it holds.
//
// The rename replaces the file at the name that it moves back to, which an
// earlier run generated after it vacated that name.
func putBackInterrupted(dirs []string, names ...string) error {
	var errs []error
	for _, dir := range dirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			backup := filepath.Join(dir, backupNameFor(name))

			if err := os.Rename(backup, path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("yama: putting %s back in %s: %w", name, dir, err))
			}
		}
	}

	return errors.Join(errs...)
}

// wireGenScopes is a set of scopes held together, one per transient file
// that a single generation run can write into a directory.
type wireGenScopes []*wireGenScope

// openWireGenScopes opens a scope over every directory, for every transient file
// name, all or nothing.
//
// A partial open is rolled back rather than returned: the directories already
// scoped would otherwise keep their originals stranded under backup names, with
// no caller that holds the scopes needed to put them back.
func openWireGenScopes(dirs []string, names ...string) (wireGenScopes, error) {
	var scopes wireGenScopes
	for _, dir := range dirs {
		for _, name := range names {
			scope, err := openWireGenScope(dir, name)
			if err != nil {
				return nil, errors.Join(err, scopes.restore())
			}
			scopes = append(scopes, scope)
		}
	}

	return scopes, nil
}

// restore restores every scope, and reports the failures together. Each is
// attempted even after an earlier one fails, so one stuck directory cannot
// strand the originals held for all the others.
func (s wireGenScopes) restore() error {
	var errs []error
	for _, scope := range s {
		if err := scope.restore(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// restore returns the directory to the state that openWireGenScope found it
// in.
//
// A backup is moved back to the name that it came from. The rename replaces
// whatever occupies that name, which is the file generated inside the scope,
// so the two steps are one. A scope over a directory that held no file of
// that name has no backup, and removes the generated file instead.
func (s *wireGenScope) restore() error {
	err := os.Rename(s.backup, s.path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("yama: restoring original %s in %s: %w; it is preserved at %s",
			s.name, s.dir, err, s.backupName)
	}

	if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("yama: removing generated %s in %s: %w", s.name, s.dir, err)
	}

	return nil
}
