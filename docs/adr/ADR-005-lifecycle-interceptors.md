# ADR-005: Lifecycle Interceptors

## Status

Accepted

## Context

Applications frequently require lifecycle-related behavior that is orthogonal to lifecycle orchestration.

Examples include:

* Logging.
* Metrics.
* Telemetry.
* Tracing.
* Diagnostics.
* Conditional lifecycle participation.
* Environment-specific lifecycle policy.

A common approach is to embed these concerns directly into the lifecycle framework.

Examples include:

* Framework-managed logging.
* Framework-managed metrics.
* Framework-managed tracing.
* Framework-managed optional components.
* Framework-managed policy engines.

These approaches expand the scope of the lifecycle manager and increase long-term maintenance burden.

The project requires an extension mechanism that allows applications to customize lifecycle behavior without increasing the responsibilities of the lifecycle manager itself.

## Decision

Lifecycle customization shall be implemented through interceptors.

Interceptors are runtime objects attached to lifecycle participants
and are the primary runtime extension mechanism.

Interceptors may:

* Observe lifecycle execution.
* Modify lifecycle execution.
* Modify context propagation.
* Suppress lifecycle execution.
* Modify lifecycle outcomes.

The lifecycle manager remains responsible only for lifecycle orchestration.

Interceptors are responsible for lifecycle customization.

## Interceptor Model

Interceptors are operation-specific.

The framework does not provide a generic interceptor interface.

Instead, the framework provides distinct interceptor contracts for each lifecycle operation.

An interceptor's signature matches the error semantics of the phase it wraps.
`Start` can fail, so its interceptor propagates an `error`. `Quiesce` and `Stop`
return nothing actionable, so their interceptors do not propagate an error. The
interceptor interfaces are therefore intentionally not uniform.

```go id="4v76xe"
type StartInterceptor interface {
    Start(ctx context.Context, next Starter) error
}
```

```go id="7nxt0o"
type QuiesceInterceptor interface {
    Quiesce(ctx context.Context, next Quiescer)
}
```

```go id="e1el5z"
type StopInterceptor interface {
    Stop(ctx context.Context, next Stopper)
}
```

The framework rejects a single shared interceptor shape. A uniform contract would
force `Quiesce` and `Stop` to carry an error return they have no use for, or force
`Start` to discard the error it must report.

The architectural requirement is that lifecycle operations remain strongly typed.

## Separate Lifecycle Chains

The lifecycle manager maintains independent interceptor chains for:

```text id="o5qh6z"
Start
Quiesce
Stop
```

A Start interceptor participates only in Start processing.

A Quiesce interceptor participates only in Quiesce processing.

A Stop interceptor participates only in Stop processing.

No operation enums are used.

No string-based dispatch is used.

## Capability-Driven Participation

Interceptor participation is determined by implemented interfaces.

Example:

```go id="4rtjlwm"
type LoggingInterceptor struct {}
```

may implement:

```text id="glvq9x"
StartInterceptor
QuiesceInterceptor
StopInterceptor
```

while:

```go id="m7zzc5"
type OptionalInterceptor struct {}
```

may implement:

```text id="8q2z0u"
StartInterceptor
```

only.

The lifecycle manager automatically includes interceptors in the chains corresponding to the interfaces they implement.

## Runtime Registration

Interceptors are runtime objects.

Interceptors are attached during lifecycle manager construction.

The framework does not generate interceptor implementations.

The framework does not generate interceptor registration.

Applications provide interceptor instances explicitly.

## Interceptor Scope

Interceptors may be attached:

```text id="8w8c11"
Globally
```

or

```text id="jlwmgo"
Per component instance
```

Global interceptors execute for all lifecycle participants.

Per-component interceptors execute only for the associated lifecycle participant.

Per-component attachment is strongly typed. Generated lifecycle construction
accepts a generated interceptors input with one field per lifecycle participant,
named for that participant. A caller scopes an interceptor to a component by
placing it in that component's generated field. There are no component names,
string keys, runtime lookup, or registration API. The concrete generated input is
defined in the architecture document.

This allows applications to apply:

```text id="s90l5q"
Telemetry
Metrics
Tracing
Logging
```

globally while applying specialized policy only to specific components.

## Ordering

Interceptor execution order is determined by registration order.

Example:

```text id="jbyey8"
Telemetry
Metrics
Logging
```

results in:

```text id="g90n9w"
Telemetry
  ↓
Metrics
  ↓
Logging
  ↓
Component
```

The lifecycle manager performs no interceptor prioritization.

The lifecycle manager performs no interceptor reordering.

## Context Propagation

Interceptors may modify context before invoking the next lifecycle participant.

Example:

```go id="6j6jmr"
ctx = context.WithValue(...)
```

and then:

```go id="5ryogc"
return next.Start(ctx)
```

Context modification is a primary mechanism for propagating lifecycle metadata.

## Lifecycle Metadata

The lifecycle manager shall populate lifecycle metadata into context before interceptor execution.

Examples include:

```text id="sd9bdk"
Component identity
Operation identity
```

This metadata enables:

* Logging.
* Metrics.
* Tracing.
* Diagnostics.

without requiring component awareness of lifecycle concerns.

The lifecycle manager is the authoritative source of lifecycle metadata.

## Behavioral Modification

Interceptors may modify lifecycle behavior.

Examples include:

* Suppressing errors.
* Skipping lifecycle execution.
* Injecting behavior.
* Modifying lifecycle outcomes.

Example:

```go id="7k4kfu"
if !enabled {
    return nil
}
```

without invoking the underlying lifecycle participant.

The lifecycle manager intentionally permits such behavior.

## Optional Components

Optional lifecycle participation is implemented through interceptors.

The lifecycle manager does not provide:

```text id="q12m4z"
Optional components
Conditional startup
Feature flags
Environment-specific lifecycle behavior
```

These concerns belong in interceptor implementations.

## Chain Construction

Interceptor chains are constructed once during lifecycle manager initialization.

The lifecycle manager does not rebuild interceptor chains during lifecycle execution.

Chain construction is a startup concern.

Lifecycle execution uses the precomputed chains.

## Universal Wrapping

Because interceptors require every lifecycle participant to be invoked through a
chain, the wrapper layer is universal. Every node is wrapped, whether or not any
interceptor is attached to it. Wrapping is not opt-in.

The observational deadline carried by the caller's context relies on this same
universal wrapping. The wrapper is where per-node overrun is detected and logged
when the deadline fires, so universal wrapping is what gives that mechanism
per-node attribution.

## Observability

Interceptors are the primary observability mechanism.

Examples include:

* Logging.
* Metrics.
* Telemetry.
* Tracing.
* Diagnostics.

The lifecycle manager itself remains observability-agnostic.

Applications select the observability tooling they require.

## Rationale

### Separation of Concerns

Lifecycle orchestration and lifecycle customization are separate concerns.

The lifecycle manager orchestrates.

Interceptors customize.

### Strong Typing

Operation-specific interceptor interfaces preserve compile-time type safety.

No string-based operation dispatch is required.

### Extensibility

Interceptors provide a controlled extension mechanism without expanding lifecycle manager responsibilities.

### Minimal Core

The lifecycle manager remains focused on:

* Ordering.
* Concurrency.
* Context propagation.
* Lifecycle execution.

Everything else can be implemented through interceptors.

## Consequences

### Positive

* Minimal lifecycle manager.
* Strong separation of concerns.
* Strong typing.
* Flexible observability integration.
* Flexible policy integration.
* No framework-managed logging or metrics.

### Negative

* Applications must explicitly provide interceptor implementations.
* Complex lifecycle customization requires additional interceptor development.

### Accepted Trade-Off

The project prefers a small orchestration core with explicit extension points over a feature-rich lifecycle framework.

## Rejected Alternatives

### Generic Interceptor Interface

Example:

```go id="xlj67e"
type Interceptor interface {
    Intercept(...)
}
```

Rejected because it weakens type safety and encourages generic dispatch logic.

### Framework-Managed Observability

Rejected because applications have diverse observability requirements.

### Framework-Managed Optional Components

Rejected because optionality is application policy rather than lifecycle orchestration.

### Interceptor Priorities

Rejected because registration order provides a simpler and more predictable execution model.

### Reflection-Based Interceptor Discovery

Rejected because the project avoids reflection and runtime registration systems.

## Non-Goals

Interceptors do not introduce:

* Retry frameworks.
* Backoff frameworks.
* Circuit breakers.
* Workflow orchestration.
* Plugin systems.
* Runtime graph modification.

Interceptors exist to customize lifecycle behavior while preserving a small lifecycle orchestration core.
