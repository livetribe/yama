package work

import (
	"errors"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// settle puts a package's files back and takes the intermediate files out of
// the directory. Every state settles this way, so no failure path leaves an
// intermediate file in the user's tree.
//
// settle returns own joined with whatever the removal produced.
func settle(c *custody.Custodian, files *wire.IntermediateYamaFiles, own error) error {
	c.Complete()

	removed := files.CleanUp()

	return errors.Join(own, removed)
}
