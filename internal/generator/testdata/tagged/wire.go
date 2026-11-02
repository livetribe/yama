// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject

package tagged

import "github.com/google/wire"

// InitRoot builds Root; NewDep is only visible under the "special" tag.
func InitRoot() *Root {
	panic(wire.Build(NewDep, NewRoot))
}
