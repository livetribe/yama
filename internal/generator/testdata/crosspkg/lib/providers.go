// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package lib

import "github.com/google/wire"

// LibSet builds the Root.
var LibSet = wire.NewSet(NewDep, NewRoot)
