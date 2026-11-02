// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package setlib states a provider set. A stub names the set, and never the
// package that declares the provider in it.
package setlib

import (
	"github.com/google/wire"

	"l7e.io/yama/internal/generator/testdata/setother"
)

var ProviderSet = wire.NewSet(setother.NewThing)
