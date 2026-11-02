// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package work

import (
	"errors"

	"l7e.io/yama/internal/generator/custody"
	"l7e.io/yama/internal/generator/wire"
)

// settle puts a package's files back. It also takes the intermediate files out
// of the directory. Every state settles this way, so no failure path leaves an
// intermediate file in the user's tree.
//
// settle joins own with the error of the restore and the error of the removal.
// It returns that joined error.
func settle(c *custody.Custodian, files *wire.IntermediateYamaFiles, own error) error {
	restored := c.Complete()

	removed := files.CleanUp()

	return errors.Join(own, restored, removed)
}
