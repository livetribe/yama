// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package yama

import "errors"

// ErrStartFailed is the single lifecycle-level error Yama exposes. Start returns
// it when startup cannot complete successfully.
//
// It carries no detail. To learn which component failed, why, or how long it
// took, use an interceptor; none of that reaches the public error surface.
var ErrStartFailed = errors.New("lifecycle start failed")
