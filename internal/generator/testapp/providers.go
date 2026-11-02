// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package testapp

import "github.com/google/wire"

// GraphSet builds the whole graph from the recorder and the fault the lifecycle
// constructor takes as arguments.
var GraphSet = wire.NewSet(NewRes, NewBase, NewMid, NewLeaf, NewTop)
