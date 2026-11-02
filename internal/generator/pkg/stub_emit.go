// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
