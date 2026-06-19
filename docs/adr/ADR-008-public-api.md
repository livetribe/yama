# ADR-008: Minimal Public API

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

* Lifecycle interfaces.
* Lifecycle capability interfaces.
* Lifecycle interceptor interfaces.
* Lifecycle-level errors.

All graph analysis, orchestration implementation, generated lifecycle structures, generated configuration structures, and generated helper types remain implementation details.

## Public Lifecycle Interface

Applications interact with lifecycle orchestration through:

```go
type Lifecycle interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

This is the primary lifecycle abstraction exposed by the framework.

Applications should not depend on generated implementation types.

Generated lifecycle implementations remain private.

## Public Lifecycle Capability Interfaces

Applications implement lifecycle behavior through:

```go
type Starter interface {
    Start(context.Context) error
}

type Drainer interface {
    Drain(context.Context) error
}

type Stopper interface {
    Stop(context.Context) error
}
```

These interfaces define lifecycle participation.

They are part of the public API.

## Public Interceptor Interfaces

Applications customize lifecycle behavior through interceptor interfaces.

Conceptually:

```go
type StartInterceptor interface {
    Start(ctx context.Context, next Starter) error
}
```

```go
type DrainInterceptor interface {
    Drain(ctx context.Context, next Drainer) error
}
```

```go
type StopInterceptor interface {
    Stop(ctx context.Context, next Stopper) error
}
```

The exact signatures are defined by the framework.

Operation-specific interceptor interfaces are part of the public API.

## Public Errors

The framework exposes lifecycle-level errors:

```go
var ErrStartFailed error
var ErrStopFailed error
```

These represent lifecycle outcomes.

The framework does not expose component-level failure information through the public error model.

## Generated Artifacts

Generated artifacts are not part of the framework's public API.

Examples include:

```go
type yamaLifecycle struct {}
type yamaLvl001 struct {}
type yamaLvl002 struct {}
```

Generated types may change as generator implementation evolves.

Applications should not depend on generated implementation details.

## Generated Configuration Types

Generated lifecycle configuration structures are application-specific generated artifacts.

Examples:

```go
type LCMConfig struct {
    ...
}
```

These structures are generated from a specific dependency graph.

They are not part of the lifecycle library's stable public API.

The lifecycle library consumes them, but they are application-specific generated artifacts rather than part of the lifecycle library's public API.

## Intentionally Omitted APIs

The framework intentionally does not expose graph concepts.

Examples:

```go
type Graph struct {}
type Node struct {}
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
