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

// backupFileMode is the permission for the placeholder that claims the backup
// name. A preserved original keeps its own mode, since it is moved, not copied.
const backupFileMode = 0o600

// backupNameFor is where a Wire output named name is held while Yama regenerates.
func backupNameFor(name string) string {
	return backupNamePrefix + name
}

// wireGenScope makes Google Wire's output transient and generation
// non-destructive.
//
// Wire writes its output into the package directory and offers no way to redirect
// it, so a file of that name already there is moved aside for the life of the scope
// and put back by restore. Only a file that appeared while the scope was open —
// Yama's own — is ever removed.
//
// The scope also serializes generation on a directory: creating the backup name is
// the exclusive claim, so a second run cannot interleave and overwrite the first
// one's preserved original.
type wireGenScope struct {
	dir        string
	name       string
	backupName string
	path       string
	backup     string
	preserved  bool
}

// openWireGenScope prepares dir for a transient Wire output named name, moving any
// existing file of that name aside.
//
// Claiming the backup name is exclusive and atomic. Testing for the name and then
// renaming onto it would be a check-then-act race: os.Rename replaces its
// destination silently, so two concurrent runs could each move a file onto the same
// backup and destroy the original. The claim fails instead, which also catches a
// backup stranded by an earlier interrupted run.
func openWireGenScope(dir, name string) (scope *wireGenScope, err error) {
	s := &wireGenScope{
		dir:        dir,
		name:       name,
		backupName: backupNameFor(name),
	}
	s.path = filepath.Join(dir, s.name)
	s.backup = filepath.Join(dir, s.backupName)

	claim, err := os.OpenFile(s.backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, backupFileMode)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("yama: %s already exists in %s, from a concurrent or interrupted run; "+
				"if it holds your original %s restore it, otherwise remove it, then regenerate",
				s.backupName, dir, s.name)
		}

		return nil, fmt.Errorf("yama: claiming %s in %s: %w", s.backupName, dir, err)
	}

	// Armed only now that the claim is ours: releasing it above would delete a
	// backup another run holds, which may be the caller's only copy.
	defer func() {
		if err != nil {
			_ = os.Remove(s.backup)
		}
	}()

	if err := claim.Close(); err != nil {
		return nil, fmt.Errorf("yama: claiming %s in %s: %w", s.backupName, dir, err)
	}

	_, statErr := os.Lstat(s.path)
	if statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("yama: inspecting %s in %s: %w", s.name, dir, statErr)
		}

		// Nothing of that name to preserve; the claim alone holds the slot.
		return s, nil
	}

	// Replaces the placeholder this scope just claimed, which nothing else holds.
	if err := os.Rename(s.path, s.backup); err != nil {
		return nil, fmt.Errorf("yama: preserving existing %s in %s: %w", s.name, dir, err)
	}
	s.preserved = true

	return s, nil
}

// wireGenScopes is a set of scopes held together, one per directory a single
// Wire invocation may write to.
type wireGenScopes []*wireGenScope

// openWireGenScopes opens a scope over every directory, all or nothing.
//
// A partial open is rolled back rather than returned: the directories already
// scoped would otherwise keep their originals stranded under backup names, with
// no caller holding the scopes needed to put them back.
func openWireGenScopes(dirs []string, name string) (wireGenScopes, error) {
	var scopes wireGenScopes
	for _, dir := range dirs {
		scope, err := openWireGenScope(dir, name)
		if err != nil {
			return nil, errors.Join(err, scopes.restore())
		}
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// restore restores every scope, reporting the failures together. Each is
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

// restore returns the directory to the state openWireGenScope found it in: the
// file generated inside the scope is removed, and any preserved original is put
// back. It releases the claim either way.
//
// Every step is attempted even after an earlier one fails, and the failures are
// reported together: giving up early would strand the original under the backup
// name when putting it back would still have succeeded.
func (s *wireGenScope) restore() error {
	var errs []error

	if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("yama: removing generated %s in %s: %w", s.name, s.dir, err))
	}

	if !s.preserved {
		if err := os.Remove(s.backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("yama: releasing %s in %s: %w", s.backupName, s.dir, err))
		}

		return errors.Join(errs...)
	}

	// Attempted even if the removal above failed: the rename replaces whatever
	// occupies the slot, so the caller's original wins over a stray generated file.
	if err := os.Rename(s.backup, s.path); err != nil {
		errs = append(errs, fmt.Errorf("yama: restoring original %s in %s: %w; it is preserved at %s",
			s.name, s.dir, err, s.backupName))
	}

	return errors.Join(errs...)
}
