// Copyright (c) 2026 the original author or authors.
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

package yama

import "errors"

// ErrStartFailed is the single lifecycle-level error Yama exposes. Start returns
// it when startup cannot complete successfully.
//
// It carries no detail. To learn which component failed, why, or how long it
// took, use an interceptor; none of that reaches the public error surface.
var ErrStartFailed = errors.New("lifecycle start failed")
