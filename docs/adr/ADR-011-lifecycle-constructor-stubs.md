# ADR-011: Lifecycle Stubs Declare the Constructor and Its Providers

## Status

Proposed

## Context

Yama emits one lifecycle constructor per orchestrated graph into the application
package (ADR-008, ADR-010). Three questions about that constructor have never had
a home.

* **Which graphs get a constructor?** A package may build several graphs. An
  application does not want every one of them orchestrated.
* **What is the constructor called, and what is its signature?** The constructor
  is the one emitted symbol an application names in its own code. Its identifier
  is application-facing in a way no other emitted identifier is.
* **How does the application state the graph?** Yama derives lifecycle ordering
  from a Google Wire injector, so something must state the providers that build
  the graph.

Deriving the name from an injector is possible. A rule such as `InitializeApp` to
`NewLifecycle` reads well on the example that motivated it. It then has to answer
for an injector named `Build`, for two injectors whose derived names collide, and
for a package that does not use the `Initialize…` convention. A derivation rule
must therefore ship with a collision rule. A collision rule disambiguates
mechanically, and `NewLifecycle2` is deterministic, unique, and meaningless to the
person who reads the call site. Derivation also makes generation all-or-nothing
over a package's injectors, because every injector that matches the rule produces
a constructor.

Google Wire met the same problem for injectors and answered it by having the
application declare them. A `wire.go` file behind a `//go:build wireinject` tag
declares each injector's name and signature. The same file states that injector's
providers with `wire.Build`. Wire fills in the body. The application writes the
declaration, and Wire generates the body only.

## Decision

An application shall declare each orchestrated graph as a **lifecycle stub**, in
a file guarded by `//go:build yamainject`. The file mirrors Google Wire's
`wireinject` convention.

Each stub declares three things and nothing else:

* the constructor's **name**, chosen by the application;
* its **signature**;
* the graph's **providers**, stated with `wire.Build` exactly as a Wire injector
  states them.

```go
//go:build yamainject

package sandbox

// NewLifecycle orchestrates the graph AppSet builds.
func NewLifecycle(opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(AppSet))
}

// NewLifecycleWithWriter orchestrates the graph CoreSet builds, taking the log
// destination as an argument.
func NewLifecycleWithWriter(w io.Writer, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(CoreSet))
}
```

### Yama derives a Wire injector from each stub

Yama writes a `wireinject`-tagged injector for every stub, runs Google Wire over
it, parses the output, and emits the constructor body. The derivation is
mechanical:

* **Name.** Derived from the stub's own name, in the `yama`-prefixed reserved
  namespace. Each stub therefore maps to one injector in Wire's output by name,
  and an injector the application declared for its own purposes carries no such
  name. The derived injector is transient, and no application code names it, so
  a mechanical name costs a reader nothing here. The objection to derived names
  applies to application-facing identifiers only.
* **Parameters.** The stub's parameters, in order, without the trailing
  `opts ...yama.Option`.
* **Results.** The stub's first result, then `func()`, then `error`. Google Wire
  returns an aggregated cleanup. The emitted constructor does not, because
  teardown runs through `Lifecycle.Stop` (ADR-007).
* **Body.** The stub's `wire.Build` call, unchanged.

The derived injector file and Wire's `wire_gen.go` are both transient. Yama
removes both. `lifecycle_gen.go` is the only committed output.

### The emitted file carries two negative tags

Yama emits `lifecycle_gen.go` guarded by `//go:build !wireinject && !yamainject`.
Each tag answers a different problem.

`!yamainject` prevents a duplicate declaration. The stub declares the constructor
under the `yamainject` tag, and the emitted file declares the same function. Yama
loads the package under that tag to read the stubs, so the emitted file must be
invisible to that load.

`!wireinject` keeps Yama's own output out of Google Wire's input. Wire
type-checks the whole package under the `wireinject` tag. A stale or broken
`lifecycle_gen.go` would otherwise fail the Wire step for a reason that has
nothing to do with Wire, and would block the regeneration that replaces it. Wire
marks its own `wire_gen.go` as `!wireinject` for the same reason.

## Rationale

### `wire.Build` keeps the meaning Google Wire gives it

Wire reads every argument to `wire.Build` as a provider, and it reads a function
argument as a provider function. A stub that names a provider set therefore says
what Wire's own rules say it says. A reader who knows Wire needs no
Yama-specific reading of the line, and no reader has to hold two meanings for one
call.

### Each orchestrated graph is declared once

A stub that states its providers is self-sufficient. An application that wants a
lifecycle constructor writes one declaration. It does not also write a Wire
injector, and it does not keep two parameter lists in agreement across two files.

### The naming problem is dissolved rather than solved

There is no derivation rule to specify, no collision rule to specify, and no
mechanical suffix to explain. Two stubs with the same name produce an ordinary Go
redeclaration error, in a file the application wrote, reported at the site of the
mistake. The application learns no Yama-specific diagnostic.

This is the reasoning ADR-007 applies to component names. Where a person reads a
name, the person who knows what it means chooses it. A framework-derived name is
unique without being informative.

### Generation is opt-in per graph

A graph with no stub gets no constructor. An application orchestrates the graphs
it wants orchestrated, one at a time. Adding a Wire injector for an unrelated
purpose adds no emitted code.

### The declaration site is where a reader looks

The stub file states, in ordinary Go, which constructors exist, what each one is
called, and which providers build each one's graph. That is the question a reader
of the application has. Answering it needs no reading of emitted output and no
knowledge of a derivation rule.

### A stub that omits the options parameter needs nothing from it

The `Option` values register interceptors and boundary components (ADR-007).
Each one configures how the lifecycle runs. None of them changes the graph.

An application that omits the parameter asks for none of that configuration.
Yama cannot tell an intentional omission from an oversight. It does not need to.
An application that later wants an interceptor adds the parameter to the stub and
generates again. Existing calls still compile, because a variadic parameter
accepts no arguments.

A mandatory parameter puts the same declaration in every stub. It then tells a
reader nothing about the constructor it is attached to.

### The stub reuses a convention this audience already knows

An application that uses Yama uses Google Wire. It already knows Wire's
`wireinject` convention: a build-tagged file, a marker function, and
`wire.Build` naming providers. The lifecycle stub reuses that file shape,
that marker function, and that kind of argument. The application learns one
new tag name, not a new file convention.

## Consequences

### Positive

* Each orchestrated graph is declared once, in one file.
* `wire.Build` means in a stub what it means in a `wire.go`.
* No derivation rule and no collision rule exist for the constructor name, so
  none has to be specified, implemented, documented, or kept stable across
  regeneration.
* The constructor's name and signature belong to the application, so they follow
  the application's own conventions.
* Generation is opt-in per graph.

### Negative

* Yama writes a second transient file into the package directory. Generation must
  preserve and restore that filename with the same care it gives `wire_gen.go`,
  so a run never destroys a file Yama does not own.
* Google Wire reports diagnostics against the derived injector, which is a file
  the application never sees. Yama must map each position back to the stub before
  it reports the error.
* An application that wants both a plain Wire injector and a lifecycle
  constructor over one graph writes two declarations. Stating the providers in a
  `wire.NewSet` variable keeps the providers themselves stated once, which is
  ordinary Wire practice.
* The application maintains a second build-tagged file beside `wire.go`.

### Accepted Trade-Off

The project accepts a second transient file, and the work of mapping Wire
diagnostics back to the stub, in exchange for one declaration per orchestrated
graph and a `wire.Build` call that keeps Wire's own meaning.

## Rejected Alternatives

### The stub names its injector instead of its providers

A stub could state `panic(wire.Build(InitializeApp))`, naming an injector already
declared in `wire.go`.

Rejected for three reasons. Wire reads a function argument as a provider
function, so that line already has a Wire meaning: it states that `InitializeApp`
provides its own first result. That meaning is not the one the stub intends, so
the form reads as Wire usage while asserting something else. The application also
declares the graph twice, once as an injector and once as a stub, and must keep
the two parameter lists in agreement. Finally, a stub of this form can only
orchestrate a graph that already has an injector, so an application that wants a
lifecycle constructor alone still writes a Wire injector it never calls.

### Derive the constructor name from the injector name

Rejected because it needs a derivation rule that holds for injector names the
project has not seen, and a collision rule whose output is unique and
uninformative. It also forces generation on every injector the rule matches.

### One constructor per package

Rejected because a package with two graphs needs two constructors. The sandbox
exemplar has exactly two. Choosing one graph as the package's lifecycle would put
the other out of Yama's reach.

### A generator flag or configuration file that lists graphs and names

Rejected because it puts application-facing names in a place that is neither the
application's Go source nor the emitted output. It also adds a configuration
format to a project that has none (ADR-007). The Go compiler checks a stub, and
it does not check a name in a flag.

## Non-Goals

This decision does not:

* Change what Yama emits into the constructor body (ADR-010).
* Change how Yama derives ordering from a parsed injector (ADR-008).
* Introduce a lifecycle registration API. A stub declares an emitted function's
  identity, and it registers no component (ADR-002).
* Give the stub file any role at runtime. Its tag excludes it from every ordinary
  build, exactly as Wire's tag excludes `wire.go`.
