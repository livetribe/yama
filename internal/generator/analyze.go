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

package generator

import (
	"fmt"
	"go/token"
	"go/types"
)

// contextPkgPath and contextTypeName name the type each capability method takes
// as its single parameter.
const (
	contextPkgPath  = "context"
	contextTypeName = "Context"
)

// capabilityMethods pairs each capability's method with the bit that it sets
// and with whether the method returns an error, which is the only way the
// three signatures differ.
var capabilityMethods = []struct {
	name         string
	bit          Capabilities
	returnsError bool
}{
	{"Start", CanStart, true},
	{"Quiesce", CanQuiesce, false},
	{"Stop", CanStop, false},
}

// errorType is the universe's error interface: the single result a Start method
// declares, and the one a Quiesce or Stop method must not.
var errorType = types.Universe.Lookup("error").Type()

// AnalysisError reports a component that Yama cannot determine the lifecycle
// capabilities of. AnalysisError names the injector and the offending source
// position. Because of this, the failure is a build-time error with a
// location, rather than a component that Yama silently treats as having no
// capability.
type AnalysisError struct {
	Injector string
	Pos      token.Position
	Msg      string
}

func (e *AnalysisError) Error() string {
	return fmt.Sprintf("yama: injector %s: %s: %s", e.Injector, e.Pos, e.Msg)
}

// Analyze resolves every component's lifecycle capabilities against the package's
// type information and computes one dependency-ordered level list per injector.
// Startup runs a list forward. Quiesce and stop run it back.
func Analyze(pkg *LoadedPackage) (*Analysis, error) {
	analysis := &Analysis{Injectors: make([]*InjectorAnalysis, 0, len(pkg.Injectors))}
	for _, inj := range pkg.Injectors {
		caps, err := detectCapabilities(pkg, inj)
		if err != nil {
			return nil, err
		}

		injectorAnalysis := computeLevels(inj, caps)
		analysis.Injectors = append(analysis.Injectors, injectorAnalysis)
	}

	return analysis, nil
}

// detectCapabilities resolves the capabilities of every component in one
// injector.
func detectCapabilities(pkg *LoadedPackage, inj *Injector) (map[*Component]Capabilities, error) {
	caps := make(map[*Component]Capabilities, len(inj.Components))
	for _, c := range inj.Components {
		typ, err := componentType(pkg, inj.Name, c)
		if err != nil {
			return nil, err
		}
		caps[c] = capabilitiesOf(typ)
	}

	return caps, nil
}

// componentType is the type of the injector-local variable the component is bound
// to, which is the type the value carries wherever generated code passes it on. A
// wire.Value or wire.InterfaceValue component is no exception: Wire declares its
// private variable without a type, so the variable takes the type of the value
// expression itself, interface binding or not.
func componentType(pkg *LoadedPackage, injector string, c *Component) (types.Type, error) {
	obj := pkg.Package.TypesInfo.Defs[c.Ident]
	if obj == nil || obj.Type() == nil {
		pos := pkg.Fset.Position(c.Ident.Pos())

		return nil, &AnalysisError{
			Injector: injector,
			Pos:      pos,
			Msg:      fmt.Sprintf("cannot resolve the type of %s", c.Name),
		}
	}

	return obj.Type(), nil
}

// capabilitiesOf reports which capability interfaces a type implements. It reads
// the type's method set, which is the one Go itself uses to decide an assignment
// to yama.Starter, yama.Quiescer, or yama.Stopper: a pointer receiver's method
// belongs to the pointer type alone, an embedded type's methods are promoted, and
// an interface contributes the methods it declares.
func capabilitiesOf(typ types.Type) Capabilities {
	methods := types.NewMethodSet(typ)

	caps := None
	for _, m := range capabilityMethods {
		if hasCapabilityMethod(methods, m.name, m.returnsError) {
			caps |= m.bit
		}
	}

	return caps
}

// hasCapabilityMethod reports whether the method set carries name with the exact
// signature the capability interface declares: one context.Context parameter, and
// a single error result when returnsError, no results otherwise. A method that
// merely shares the name is not a capability.
func hasCapabilityMethod(methods *types.MethodSet, name string, returnsError bool) bool {
	sel := methods.Lookup(nil, name)
	if sel == nil {
		return false
	}

	sig, ok := sel.Type().(*types.Signature)
	if !ok || !takesContext(sig.Params()) {
		return false
	}

	results := sig.Results()
	if !returnsError {
		return results.Len() == 0
	}

	return results.Len() == 1 && types.Identical(results.At(0).Type(), errorType)
}

// takesContext reports whether params is exactly one context.Context.
func takesContext(params *types.Tuple) bool {
	if params.Len() != 1 {
		return false
	}

	typ := types.Unalias(params.At(0).Type())
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == contextPkgPath && obj.Name() == contextTypeName
}
