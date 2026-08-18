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

package pkg

// defaultOptionsName is the name that a constructor uses to forward options,
// when the stub bound no name to that parameter.
const defaultOptionsName = "opts"

// OptionsName returns the name that the stub's constructor uses to forward its
// options. It is empty for a stub that declares no options.
//
// A stub can leave that parameter unnamed. A stub can also bind that parameter
// to the blank identifier. The constructor then forwards the options under a
// name of its own.
func (s *Stub) OptionsName() string {
	if !s.HasOpts {
		return ""
	}

	last := s.Params[len(s.Params)-1]
	if last.Name == "" || last.Name == blankName {
		return defaultOptionsName
	}

	return last.Name
}
