// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package nostubs is a fixture for the CLI's silent-skip behavior: it declares
// no lifecycle stub, so a run that names it generates nothing.
package nostubs

// Root is an ordinary type the package declares. Nothing builds it.
type Root struct{}
