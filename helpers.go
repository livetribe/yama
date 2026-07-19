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

import "os"

// RunUntilSignal is the typical main entry point for a Yama application: it
// starts lc, blocks until one of the given OS signals is delivered, then calls
// lc.Stop and returns. If Start fails it returns an error matching ErrStartFailed
// without waiting for a signal.
func RunUntilSignal(lc Lifecycle, signals ...os.Signal) error {
	// Not implemented yet — the body, including the default signal set used when
	// signals is empty, lands with the implementation. Panic rather than return a
	// misleading nil so an accidental early call is loud rather than wrong.
	_ = lc
	_ = signals
	panic("RunUntilSignal is not implemented")
}
