# ADR-007: Minimal Public API

## Status

Accepted

## Context

The project is intentionally designed as a focused lifecycle orchestration library.

The project is not:

* A dependency injection framework.
* A runtime lifecycle engine.
* A workflow orchestrator.
* An application framework.
* A configuration framework.

A key architectural goal is to minimize the public API surface area.

Every public type, interface, function, and abstraction becomes a long-term compatibility commitment.

The framework should expose only those concepts that application developers must directly interact with.

Everything else should remain:

* Generated.
* Private.
* Internal.
* Replaceable.

## Decision

The framework shall expose a deliberately small public API.

The public API consists of:

* The `Lifecycle` type. Its constructor is generated into the application's own
  package and named by the application (ADR-011), so it is not a framework symbol.
* Lifecycle capability interfaces.
* Lifecycle interceptor interfaces.
* The lifecycle-level error.
* A small set of lifecycle helpers, and the `Option` type they construct.

All graph analysis, orchestration implementation, generated lifecycle structures, and generated helper types remain implementation details.

The listings below show the shape of each part of that surface and the reasoning for it. They are not the catalog of it: the authoritative enumeration is the Architecture document's Public API Reference, which is the document to consult and to update when the surface changes.

## Public Lifecycle Type

Applications interact with lifecycle orchestration through `Lifecycle`, an
interface composed of the capability interfaces its own components implement:

```go
type Lifecycle interface {
    Starter
    Stopper
}
```

`Start` returns an error; `Stop` returns nothing. `Quiesce` is not exposed —
`Stop` runs the quiesce pass internally as its first action.

A `Lifecycle` starts and stops the whole graph, so it is the same kind of thing as
the components inside it: a `Starter` and a `Stopper`. Expressing it as the
composition of those two interfaces states that directly rather than restating
their method signatures a second time.

The implementation is private and owned by the runtime-support package.
Applications receive a `Lifecycle`; they do not implement or construct one, and no
public construction path exists.

An interface adds no compatibility commitment beyond the one already made.
`Starter` and `Stopper` are public and frozen, so composing them introduces no
method Yama was not already committed to. The freedom a concrete type would
preserve — adding a method to `Lifecycle` later — is freedom this project has
already renounced: ADR-003 Invariant 6 fixes the phase count at three, and the
sections below reject runtime graph, observability, and configuration APIs. There
is nothing left to add.

`Lifecycle` is named without a `-Container` suffix. The type is the application's
lifecycle, not a dependency-injection container.

The generated constructor builds the application and its `Lifecycle` together and
returns them, mirroring Google Wire's `(T, func(), error)` convention:

```go
app, lifecycle, err := NewLifecycle(WithInterceptors(i1, i2), WithBeginComponents(c1), WithEndComponents(c2))
```

The constructor takes no lifecycle configuration. Yama generates none. Any deadline
comes from the context the caller passes to `Start` and `Stop`; per-component timeouts
are component-authored wrappers. The only construction-time inputs are the
`Option` values — interceptors and boundary-component registration — described
under Public Helpers below.

On construction failure it returns `nil, nil, err` — there is no partial
`Lifecycle`, inheriting Google Wire's failure semantics. There is no cleanup
function for the caller to hold either: the constructor re-emits the injector's
construction rather than calling it (ADR-008), so Google Wire never aggregates one,
and each provider's cleanup is routed to its own position in the teardown ordering.
Teardown runs through `Lifecycle.Stop` and nowhere else.

This is the primary lifecycle abstraction exposed by the framework.

Applications should not depend on other generated implementation types.

Generated lifecycle implementation details remain private.

## Public Lifecycle Capability Interfaces

Applications implement lifecycle behavior through:

```go
type Starter interface {
    Start(context.Context) error
}

type Quiescer interface {
    Quiesce(context.Context)
}

type Stopper interface {
    Stop(context.Context)
}
```

These interfaces define lifecycle participation.

They are part of the public API.

Their signatures follow the error semantics of the phase each represents. `Start`
can fail and returns an error. `Quiesce` and `Stop` are shutdown operations with
nothing actionable to return and so return no error.

## Public Interceptor Interfaces

Applications customize lifecycle behavior through interceptor interfaces.

Conceptually:

```go
type StartInterceptor interface {
    Start(ctx context.Context, next Starter) error
}
```

```go
type QuiesceInterceptor interface {
    Quiesce(ctx context.Context, next Quiescer)
}
```

```go
type StopInterceptor interface {
    Stop(ctx context.Context, next Stopper)
}
```

The interceptor interfaces are intentionally not uniform. Each matches the error
semantics of the phase it wraps: `StartInterceptor` propagates an `error`, while
`QuiesceInterceptor` and `StopInterceptor` do not.

Operation-specific interceptor interfaces are part of the public API.

## Public Context Accessor

Interceptors read the lifecycle component they are wrapping through a single
accessor:

```go
func FromContext[T any](ctx context.Context) (T, bool)
```

The lifecycle manager attaches the component itself to the context before the
interceptor chain runs; an interceptor recovers it with `FromContext`.
This is part of the public API.

The accessor yields the component, not a framework-owned descriptor of it. An
interceptor cannot obtain the component from its `next` argument — `next` is the
rest of the chain, so only the final link ever holds the component — which is why
the context carries it.

`T` is the component's concrete type. Because interceptors attach globally
(ADR-005), an interceptor uses `any` and type-switches to identify the
component it is wrapping. `T` is unconstrained because Go cannot express
"implements at least one of `Starter`, `Quiescer`, `Stopper`": a union may not
contain method-bearing interfaces, and embedding them would require all three
rather than any one. A base interface does not rescue this — an empty one
constrains nothing, an unexported marker method would make the capability
interfaces unimplementable outside `package yama`, and an exported marker would
impose boilerplate on every component while still not proving the type
participates in the lifecycle. This is the same limitation that makes
`WithBeginComponents` take `any`.

**Components are not named by the framework.** Yama derives no component name
and exposes none. A component that wants a printable identity implements
`fmt.Stringer`; otherwise `%T` yields its type. This is ordinary Go, and it is
strictly better than a generated name: the framework could only derive a name from
the shape of the Wire graph, and would have to disambiguate two same-typed
components with a mechanical suffix — `sqlDB` and `sqlDB2` — which is unique but
tells an operator nothing. A `String()` returning `"replica-db"` is chosen by the
person who knows what it means. Every component implements a capability
interface, so it is always a type the application owns and can extend.

The operation being performed (Start, Quiesce, or Stop) is **not** carried as
context metadata. Because those are separate interceptor methods, an interceptor
already knows which operation it is handling from the method that was invoked, so
no operation-identity accessor exists.

The accessor exposes the component only. It does not expose graph APIs,
generated implementation types, lifecycle plans, or component error details.

## Public Errors

The framework exposes a single lifecycle-level error:

```go
var ErrStartFailed error
```

`Start` is the only lifecycle operation that returns an error. `Stop` returns
nothing, so there is no `ErrStopFailed`.

The framework does not expose component-level failure information through the public error model.

## Public Helpers

The framework provides a small set of helpers for common lifecycle patterns:

```go
func RunUntilSignal(lc Lifecycle, signals ...os.Signal) error // Start, wait for a signal, then Stop
func WithBeginComponents(components ...any) Option // base-extreme: start before the graph, tear down after it
func WithEndComponents(components ...any) Option   // top-extreme: start after the graph, tear down before it
func WithInterceptors(interceptors ...any) Option  // attach interceptors globally
```

`RunUntilSignal` is the typical `main` entry point: it starts, waits for the
signal, and calls `Stop()`. `WithBeginComponents`, `WithEndComponents`, and `WithInterceptors`
register construction-time inputs as `Option`s, keeping that registration
out of any generated or runtime API. All three are variadic and may each be
passed more than once; supplied values accumulate. For `WithInterceptors`,
accumulation order is the interceptor chain's registration order (ADR-005). For
`WithBeginComponents`/`WithEndComponents`, each boundary is a flat, unordered set — call
order carries no ordering guarantee among components registered into the same
boundary (Architecture §18). There is no generated, per-application variant of
any of these helpers. Their exact signatures are defined by the framework.

A `Start` that would otherwise block (for example `http.Server.ListenAndServe`)
is the component's own responsibility to launch in a goroutine and return; the
framework provides no helper for it. Likewise, idempotent shutdown is the
framework's own guarantee — `Stop` runs its passes once — so components needing a
once-only "stop accepting new work" step use ordinary `sync.Once`.

## Generated Artifacts

Generated artifacts are not part of the framework's public API.

The generated file emits no types (ADR-010). Its only symbol is the lifecycle
constructor, and that one is application-facing by design: the application
declares its name and signature itself (ADR-011) and calls it directly.

```go
app, lifecycle, err := NewLifecycle(WithInterceptors(i1, i2))
```

That constructor is part of the *application's* surface, not the framework's.
Nothing else generated is reachable, and the shape of what is generated —
including how the constructor's body declares its levels — may change as the
generator evolves.

Applications should not depend on generated implementation details.

## Non-Public Implementation Symbols

Generated code may call additional Yama-owned symbols that are exported only so
generated application-package code can reach them, for execution plumbing the
generator does not emit inline. Such symbols are **not** part of the stable public
API defined by this ADR, and applications should not depend on them directly. The
stable public surface remains only the types listed above: the capability
interfaces, the interceptor interfaces, `FromContext`, `ErrStartFailed`,
`Lifecycle`, `Option`, and the small set of helpers. The lifecycle constructor is
not among them: it is generated into the application's own package under a name
the application chose (ADR-011).

## Intentionally Omitted APIs

The framework intentionally does not expose graph concepts.

Examples:

```go
type Graph struct {}
type Component struct {}
type Plan struct {}
type ExecutionGroup struct {}
type Level struct {}
```

do not exist in the public API.

The dependency graph is a generation-time concern.

The generated code is the resulting artifact.

## No Registration APIs

The framework does not expose:

```go
Register(...)
AddComponent(...)
Build(...)
```

The framework does not construct dependency graphs.

Google Wire remains responsible for graph construction.

## No Runtime Graph APIs

The framework does not expose:

```go
Graph()
Describe()
Plan()
ExecutionGroups()
```

Applications inspect generated code rather than querying runtime lifecycle metadata.

## No Observability APIs

The framework does not expose:

```go
SetLogger(...)
SetTracer(...)
SetMeter(...)
```

Observability is implemented through interceptors.

The framework remains observability-agnostic.

## No Configuration APIs

The framework does not expose:

```go
LoadConfig(...)
ReloadConfig(...)
ParseYAML(...)
ParseJSON(...)
```

Configuration management remains outside the scope of lifecycle orchestration.

## Rationale

### Minimize Compatibility Burden

Every public API becomes a long-term support commitment.

A small API is easier to evolve and maintain.

### Preserve Implementation Freedom

By keeping generated artifacts private, generator internals can evolve without affecting applications.

### Reduce Concept Count

The framework should expose only concepts applications must understand.

Internal implementation concepts should remain hidden.

### Alignment with Project Philosophy

The project consistently prefers:

* Compile-time behavior.
* Generated code.
* Explicitness.
* Simplicity.

A minimal public API supports those goals.

## Consequences

### Positive

* Small API surface.
* Reduced maintenance burden.
* Easier evolution of generated code.
* Reduced conceptual complexity.
* Strong separation between API and implementation.

### Negative

* Some internal lifecycle concepts are not directly accessible.
* Applications must rely on generated code rather than runtime introspection.

### Accepted Trade-Off

The project prioritizes maintainability and simplicity over exposing additional framework capabilities.

## Rejected Alternatives

### Public Lifecycle Graph APIs

Rejected because lifecycle graphs are generation-time artifacts.

### Public Runtime Plans

Rejected because generated code is the authoritative execution model.

### Public Generated Types

Rejected because generated implementation details should remain replaceable.

### Framework-Owned Observability APIs

Rejected because observability requirements vary by application.

### Framework-Owned Configuration APIs

Rejected because configuration management is a separate concern.

## Non-Goals

The public API does not provide:

* Dependency graph access.
* Runtime lifecycle plans.
* Registration systems.
* Configuration frameworks.
* Observability frameworks.
* Runtime graph inspection.

The framework exposes only the interfaces required to participate in lifecycle orchestration and customization.
