// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
