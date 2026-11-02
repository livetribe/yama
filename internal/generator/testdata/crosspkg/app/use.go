// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"l7e.io/yama"

	"l7e.io/yama/internal/generator/testdata/crosspkg/lib"
)

// StartLib calls the constructor the sibling package's stub declares. This file
// carries no build tag.
func StartLib() (*lib.Root, yama.Lifecycle, error) {
	return lib.NewLibLifecycle()
}
