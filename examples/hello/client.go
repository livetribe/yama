// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package hello

import (
	"fmt"
	"io"
)

// Client stands in for a type from a package that the application does not
// own. It declares Close and no lifecycle capability. The closer directive on
// its provider binds Close to the teardown pass.
type Client struct {
	w     io.Writer
	store *Store
}

// NewClient provides the Client. It depends on the Store, so it closes before
// the store stops.
//
//yama:closer
func NewClient(w io.Writer, store *Store) *Client {
	return &Client{w: w, store: store}
}

// Close releases what the client holds.
func (c *Client) Close() error {
	fmt.Fprintln(c.w, "client: closed")

	return nil
}
