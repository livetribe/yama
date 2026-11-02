// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package noinjector

// Plain has no provider and no injector: this package is not a Wire package at
// all, and a sweep across it must skip it silently rather than error.
type Plain struct{}
