// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
