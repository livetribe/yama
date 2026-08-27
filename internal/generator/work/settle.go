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
