// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package typeerror

// App is the value the injector builds.
type App struct{}

func NewApp() *App { return &App{} }

// Broken does not compile. A load of this package reports it, and every type in
// the package is then unresolved.
func Broken() int { return "not an int" }
