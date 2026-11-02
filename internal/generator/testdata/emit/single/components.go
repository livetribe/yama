// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package single

import "context"

// One component, which is both the graph and its root.
type Only struct{}

func NewOnly() *Only { return &Only{} }

func (*Only) Start(context.Context) error { return nil }

func (*Only) Stop(context.Context) {}
