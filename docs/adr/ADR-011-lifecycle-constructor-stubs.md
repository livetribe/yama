# ADR-011: Lifecycle Constructor Stubs Declare the Generated Surface

## Status

Accepted

## Context

Yama emits one lifecycle constructor per injector into the application package
(ADR-008, ADR-010). Two questions about that constructor have never had a home:

* **Which injectors get one?** A package may hold several injectors, and not
  every injector's graph is one an application wants orchestrated.
* **What is it called, and what is its signature?** The constructor is the one
  generated symbol an application names in its own code, so its identifier is
  application-facing in a way no other generated identifier is.

Deriving the answer from the injector is possible. A rule such as
`InitializeApp` → `NewLifecycle` reads well on the example that motivated it and
then has to answer for everything else: an injector named `Build`, two injectors
whose derived names collide, a package whose naming convention is not
`Initialize…`. A derivation rule must therefore ship with a collision rule, and a
collision rule mechanically disambiguates — `NewLifecycle2` — which is
deterministic, unique, and meaningless to the person reading the call site.
Deriving also makes generation all-or-nothing over a package's injectors, since
every injector matching the rule produces a constructor whether or not one was
wanted.

Google Wire faced the same problem for injectors themselves and answered it by
having the application declare them: a `wire.go` behind a `//go:build wireinject`
tag declares each injector's name and signature and states its providers with
`wire.Build`, and Wire fills in the body. The declaration is hand-authored; only
the body is generated.

## Decision

An application shall declare its lifecycle constructors as **stubs**, in a file
guarded by `//go:build yamainject`, mirroring Google Wire's `wireinject`
convention. Yama emits the bodies into `lifecycle_gen.go`, guarded by
`//go:build !wireinject && !yamainject`.

Each stub declares three things and nothing else:

* the constructor's **name**, chosen by the application;
* its **signature**;
* the **injector** whose graph it orchestrates, named in the stub body as
  `panic(wire.Build(InjectorName))`.

```go
//go:build yamainject

package sandbox

// NewLifecycle orchestrates the graph InitializeApp builds.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(InitializeApp))
}

// NewLifecycleWithWriter orchestrates the graph InitializeAppWithWriter builds,
// mirroring that injector's io.Writer argument.
func NewLifecycleWithWriter(w io.Writer, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(InitializeAppWithWriter))
}
```

A stub's signature is derived from its injector's, mechanically and checkably:

* **Parameters** are the injector's parameters, in order, followed by
  `opts ...yama.Option`. The generated body threads each parameter through the
  re-emitted construction in place of whatever the injector's own body used.
* **Results** are the injector's first result, then `yama.Lifecycle`, then
  `error`. Google Wire's aggregated `func()` cleanup result has no counterpart:
  teardown runs through `Lifecycle.Stop`, so the constructor does not return a
  cleanup for a caller to invoke (ADR-007).

A signature that does not match its injector's is a generation-time error,
reported against the stub.

Because the file is excluded from every ordinary build by its tag, `wire.Build`
here is read by Yama and never executed or seen by Google Wire's own generator.
Its role in a stub is to name the injector, which is the same role it plays in a
`wire.go` — naming what the generated body is built from.

## Rationale

### Generation becomes opt-in per injector

An injector with no stub gets no constructor. An application orchestrates the
graphs it wants orchestrated, one at a time, and adding an injector for some
unrelated purpose does not silently add generated code.

### The naming problem is dissolved rather than solved

There is no derivation rule to specify, no collision rule to specify, and no
mechanical suffix to explain. Two stubs with the same name are an ordinary Go
redeclaration error in a file the application wrote, reported by the compiler at
the site of the mistake, with no Yama-specific diagnostic to learn.

This is the same reasoning ADR-007 applies to component names: where a name is
going to be read by a person, the person who knows what it means chooses it, and
a framework-derived name is unique without being informative.

### The declaration site is where a reader looks

The stub file states, in ordinary Go, which constructors exist, what they are
called, and what each is built from. That is the question a reader of the
application has — and answering it does not require reading generated output or
knowing a derivation rule.

### It is a convention this audience already knows

An application using Yama is by definition using Google Wire, and therefore
already has a `wireinject`-tagged file that declares stubs whose bodies are
generated. The mechanism is the one already in the project, applied to a second
generated artifact.

## Consequences

### Positive

* Generation is opt-in per injector.
* No name derivation rule and no collision resolution rule exist to specify,
  implement, document, or keep stable across regeneration.
* The constructor's name and signature are the application's, so they can follow
  the application's own conventions.
* The signature check gives a locatable build-time error where a derived
  signature would have silently produced a constructor the application did not
  expect.

### Negative

* An application maintains a second hand-authored, build-tagged file beside
  `wire.go`, and adding an injector it wants orchestrated is two edits rather
  than one.
* The stub duplicates the injector's parameter list, so an injector signature
  change requires a matching stub edit. The signature check turns that into a
  build error rather than a silent divergence, but it is still an edit.
* `wire.Build` is used as a marker in a file Google Wire never processes, which
  reads as Wire usage without being it.

### Accepted Trade-Off

The project accepts a second declaration file, and the duplication of each
injector's parameter list within it, in exchange for application-chosen
constructor names, per-injector opt-in, and the removal of a derivation-plus-
collision rule from the generator.

## Rejected Alternatives

### Derive the constructor name from the injector name

Rejected because it requires a derivation rule that holds for injector names the
project has not seen, plus a collision rule whose output is unique and
uninformative. It also forces generation on every injector the rule matches.

### One constructor per package

Rejected because a package with two injectors — the sandbox exemplar has exactly
that — needs two constructors, and picking one injector as the package's
lifecycle would make the other unreachable through Yama.

### A generator flag or configuration file listing injectors and names

Rejected because it puts application-facing names in a place that is neither the
application's Go source nor the generated output, and adds a configuration format
to a project that has none (ADR-007). A Go stub is checked by the compiler; a
name in a flag is not.

## Non-Goals

This decision does not:

* Change what is generated into the constructor body (ADR-010) or how ordering is
  derived (ADR-008).
* Introduce a lifecycle registration API — a stub declares a generated function's
  identity, and registers no component (ADR-002).
* Give the stub file any role at runtime; it is excluded from every ordinary
  build, exactly as `wire.go` is.
