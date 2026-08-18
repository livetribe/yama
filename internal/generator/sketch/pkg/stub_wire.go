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

// GraphParams returns the parameters that the derived injector takes. These are
// the stub's own parameters, without a trailing variadic option parameter.
func (s *Stub) GraphParams() []Field {
	if !s.HasOpts {
		return s.Params
	}

	return s.Params[:len(s.Params)-1]
}

// ResultType returns the type of the value that the stub builds. That type is
// the type of the stub's first result.
func (s *Stub) ResultType() string {
	return s.Results[0].Type
}

// WireName returns the name that the stub's own file uses for Google Wire. A
// stub that states no name takes the name that the path itself states.
func (s *Stub) WireName() string {
	return PackageName(s.Wire, WirePackagePath)
}
