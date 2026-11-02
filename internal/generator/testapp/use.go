// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package testapp

import "l7e.io/yama"

// Start calls the constructor this package's stub declares, and returns what it
// returns. This file carries no build tag.
func Start(rec *Recorder, fault Fault, opts ...yama.Option) (*Top, yama.Lifecycle, error) {
	return NewLifecycle(rec, fault, opts...)
}
