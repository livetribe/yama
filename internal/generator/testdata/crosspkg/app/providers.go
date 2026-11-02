// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package app

import "github.com/google/wire"

// AppSet builds the App.
var AppSet = wire.NewSet(NewSvc, NewApp)
