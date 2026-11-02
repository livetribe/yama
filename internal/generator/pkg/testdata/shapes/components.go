// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package shapes

import "database/sql"

// An App is the value that every stub in this package builds.
type App struct{}

// NewApp is the one provider that every stub states.
func NewApp(db *sql.DB) *App {
	return &App{}
}
