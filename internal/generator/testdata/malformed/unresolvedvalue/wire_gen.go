// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

package unresolvedvalue

// config reads a name the file declares no value for. The emitted constructor
// cannot reproduce that read, because the file declaring it is removed.
func InitializeApp() (*App, error) {
	config := elsewhereValue
	root := NewRoot(config)
	return root, nil
}
