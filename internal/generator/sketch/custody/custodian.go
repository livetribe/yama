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

package custody

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// BackupPrefix starts the name that a Custodian moves a file to. The Go
// toolchain does not read a file whose name starts with a dot, so a backup sits
// beside a file of the live name without a collision.
const BackupPrefix = ".yama."

// The two names a run settles. The run's output-file prefix starts both.
const (
	lifecycleFile = "lifecycle_gen.go"
	wireFile      = "wire_gen.go"
)

// A Custodian settles the two generated files of one package directory. See
// custody.md. The row numbers below name the rows of its two tables.
type Custodian struct {
	path   string
	prefix string

	backupError bool
}

func NewCustodian(path, prefix string) *Custodian {
	return &Custodian{path, prefix, false}
}

// SetAside puts both generated files out of a run's way. It runs before Google
// Wire. For each name:
//
//	file   backup   action                          rows
//	no     no       nothing                         1, 5
//	no     yes      nothing                         2, 6
//	yes    no       move the file to the backup     3, 7
//	yes    yes      keep the backup, delete file    4, 8
//
// A backup that is already present belongs to a run that did not finish. That
// backup holds the file the user owns, and the live name holds what the dead
// run produced. SetAside keeps the backup.
//
// SetAside tries both names, and it joins what they returned. A failure of
// either one records that this run did not take custody of the directory.
// Complete reads that record.
func (c *Custodian) SetAside() error {
	var errs []error

	for _, name := range c.names() {
		err := c.setAside(name)
		errs = append(errs, err)
	}

	joined := errors.Join(errs...)
	if joined != nil {
		c.backupError = true
	}

	return joined
}

// Complete settles both generated files. It runs after every package generated.
//
// The lifecycle file is Yama's, so a file at the live name wins:
//
//	file   backup   action                 rows
//	no     no       nothing                9
//	no     yes      restore the backup     10
//	yes    no       nothing                11
//	yes    yes      delete the backup      12
//
// Google Wire's output name is the application's, so the backup wins:
//
//	file   backup   action                              rows
//	no     no       nothing                             13
//	no     yes      restore the backup                  14
//	yes    no       delete the file, unless SetAside    15
//	                failed
//	yes    yes      replace the file with the backup    16
//
// Row 15 is the one row that the directory cannot answer on its own. A live
// Wire output with no backup is what Google Wire wrote when SetAside succeeded.
// It is the file the application owns when SetAside failed, and Complete leaves
// that file alone.
//
// Complete tries both names, and it joins what they returned. A failed restore
// leaves a file the user owns at the backup name, and the error that Complete
// returns states that name.
func (c *Custodian) Complete() error {
	emitted := c.completeEmitted(c.prefix + lifecycleFile)
	borrowed := c.completeBorrowed(c.prefix + wireFile)

	return errors.Join(emitted, borrowed)
}

// WireOutputName returns the name that Google Wire writes in this directory.
func (c *Custodian) WireOutputName() string {
	return c.prefix + wireFile
}

// LifecycleFileName returns the name that Yama writes in this directory.
func (c *Custodian) LifecycleFileName() string {
	return c.prefix + lifecycleFile
}

// names returns both generated filenames, with the run's prefix on each.
func (c *Custodian) names() []string {
	return []string{c.prefix + lifecycleFile, c.prefix + wireFile}
}

// setAside implements rows 1 to 8 for one name.
func (c *Custodian) setAside(name string) error {
	live, backup := c.paths(name)

	if !present(live) {
		return nil
	}

	if present(backup) {
		err := os.Remove(live)

		return wrapPath("set aside", name, c.path, err)
	}

	err := os.Rename(live, backup)

	return wrapPath("set aside", name, c.path, err)
}

// completeEmitted implements rows 9 to 12 for a name that Yama writes.
func (c *Custodian) completeEmitted(name string) error {
	live, backup := c.paths(name)

	if !present(backup) {
		return nil
	}

	if present(live) {
		err := os.Remove(backup)

		return wrapPath("drop the backup of", name, c.path, err)
	}

	err := os.Rename(backup, live)

	return c.wrapRestore(name, err)
}

// completeBorrowed implements rows 13 to 16 for a name that Yama borrows.
func (c *Custodian) completeBorrowed(name string) error {
	live, backup := c.paths(name)

	if present(backup) {
		err := os.Rename(backup, live)

		return c.wrapRestore(name, err)
	}

	if !present(live) || c.backupError {
		return nil
	}

	err := os.Remove(live)

	return wrapPath("remove", name, c.path, err)
}

// wrapRestore names the file, the directory, and the backup that still holds
// the file the user owns.
func (c *Custodian) wrapRestore(name string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("restore %s in %s: %w; it is preserved at %s", name, c.path, err, BackupPrefix+name)
}

// paths returns the live path and the backup path for name.
func (c *Custodian) paths(name string) (live, backup string) {
	live = filepath.Join(c.path, name)
	backup = filepath.Join(c.path, BackupPrefix+name)

	return live, backup
}

// present reports whether path names an entry in the filesystem.
func present(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

// wrapPath names the operation, the file, and the directory when err is not
// nil.
func wrapPath(op, name, dir string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s %s in %s: %w", op, name, dir, err)
}
