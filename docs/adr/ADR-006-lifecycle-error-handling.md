# ADR-006: Lifecycle Error Model

## Status

Accepted

## Context

Lifecycle operations may fail for many reasons.

Examples include:

* Component startup failures.
* Component shutdown failures.
* Context cancellation.
* Context deadline expiration.
* Resource exhaustion.
* Dependency failures.

A common approach is to propagate component errors through the lifecycle system.

Examples include:

```go
type LifecycleError struct {
    Component string
    Operation string
    Cause     error
}
```

or:

```go
errors.Join(...)
```

containing multiple component failures.

These approaches expose lifecycle implementation details to callers and encourage applications to couple business logic to lifecycle internals.

The project requires an error model that:

* Remains simple.
* Remains stable.
* Preserves separation of concerns.
* Avoids turning lifecycle orchestration into a diagnostics framework.

## Decision

The lifecycle manager shall expose lifecycle-level errors only.

The lifecycle manager shall not expose:

* Component identities.
* Component error values.
* Component failure details.
* Timeout details.
* Joined errors.
* Error aggregation structures.

The lifecycle manager reports lifecycle outcomes.

Detailed diagnostics belong in interceptors and observability systems.

## Public Error Surface

The public lifecycle error surface consists of a single error:

```go
var ErrStartFailed = errors.New("lifecycle start failed")
```

`Start` is the only lifecycle operation that returns an error. `Stop` returns
nothing. There is no `ErrStopFailed`, no `ErrQuiesceFailed`, and no timeout-specific
error.

No additional lifecycle errors are exposed.

## Startup Errors

If startup cannot be completed successfully:

```go
err := lifecycle.Start(ctx)
```

returns:

```go
ErrStartFailed
```

The caller is informed that lifecycle startup failed.

The caller is not informed which component failed.

The caller is not informed why the component failed.

Those details belong in observability systems.

## Shutdown Errors

Shutdown returns nothing:

```go
lifecycle.Stop(ctx)
```

There is no recovery from shutdown, so there is nothing to return. `Stop` runs the
quiesce pass and then teardown, in dependency order, to completion. It does not
report per-component success or failure to the caller.

Component-level shutdown problems are observed through interceptors and
observability, not through a return value.

## Quiesce Errors

Quiesce is the first pass of `Stop`. Applications do not invoke it directly.

`Quiescer.Quiesce` returns no error at all. Quiesce has no public error surface
and no framework-visible failure mode. There is no `ErrQuiesceFailed` and no
`ErrDrainFailed`.

## Timeout Errors

There are no timeout errors.

The shutdown deadline is observational. When it fires, the framework logs that a
component exceeded its window and continues waiting for the operation to
complete; it does not return a timeout error and does not abandon the traversal.

A start that exceeds its deadline is handled as an ordinary start failure and
surfaces as `ErrStartFailed`. The lifecycle manager does not distinguish a start
timeout from any other start failure at the public API level.

## Startup Failure Cleanup

If startup fails after one or more components have successfully started:

```text
Start
  ↓
Failure
  ↓
Stop (quiesce pass, then teardown)
  ↓
ErrStartFailed
```

The lifecycle manager runs the same internal shutdown sequence `Stop` performs,
scoped to the successfully started components.

Shutdown produces no error, so it does not alter the public error returned.

The caller receives:

```go
ErrStartFailed
```

because the lifecycle operation that failed was startup.

Detailed cleanup diagnostics belong in observability systems.

## Shutdown Always Completes

Shutdown runs the quiesce pass and the teardown pass in dependency order, to
completion. Neither pass returns an error, and the traversal is never abandoned to
reclaim liveness.

Because the framework waits for each component rather than returning early, a
hung component stalls everything after it in the traversal until the orchestrator
sends SIGKILL. This is an accepted consequence of preserving reverse-topological
ordering. External liveness is bounded by SIGKILL, not by a returned error.

After shutdown completes, nothing is returned. Component-level shutdown outcomes
are available only through interceptors and observability.

## Diagnostics

Detailed diagnostics are intentionally separated from lifecycle outcomes.

Examples of diagnostic information include:

* Component identity.
* Duration.
* Timeout occurrence.
* Original component error.
* Dependency context.

This information is provided through interceptors and observability integrations.

The lifecycle manager itself does not expose this information.

## Rationale

### Separation of Concerns

Lifecycle orchestration and diagnostics are separate responsibilities.

The lifecycle manager determines:

```text
Did startup succeed?
```

Observability systems determine:

```text
Why did it fail?
Which component failed?
What error occurred?
```

### Stable API

Lifecycle-level errors remain stable even as component implementations evolve.

The lifecycle manager does not leak implementation details into public APIs.

### Simplicity

Most callers can only make one meaningful decision:

```go
if err != nil {
    ...
}
```

Detailed component-level error handling rarely influences lifecycle behavior.

Exposing those details increases complexity without providing proportional value.

### Observability-First Design

Component-level diagnostics are more useful when:

* Logged.
* Traced.
* Measured.
* Alerted upon.

rather than embedded in lifecycle return values.

## Consequences

### Positive

* Minimal public API.
* Stable error model.
* Clear separation between orchestration and diagnostics.
* No error aggregation complexity.
* No component identity leakage.

### Negative

* Callers cannot inspect component failures through lifecycle return values.
* Applications must use observability tooling to diagnose failures.

### Accepted Trade-Off

The project prioritizes operational simplicity and observability over detailed lifecycle return values.

## Rejected Alternatives

### Lifecycle Error Hierarchies

Example:

```go
type LifecycleError struct {
    Component string
    Operation string
    Cause     error
}
```

Rejected because it exposes lifecycle implementation details through the public API.

### Joined Errors

Example:

```go
errors.Join(...)
```

Rejected because callers rarely perform meaningful remediation based on individual lifecycle failures.

### Component Error Propagation

Example:

```go
return componentErr
```

Rejected because component errors are implementation details.

Lifecycle callers are interested in lifecycle outcomes.

### Timeout-Specific Errors

Example:

```go
ErrStartTimeout
ErrStopTimeout
```

Rejected because the framework does not return timeout errors at all. A start
timeout is an ordinary start failure surfaced as `ErrStartFailed`, and the
shutdown deadline is observational rather than an error. Timeout diagnostics
belong in observability systems.

## Non-Goals

The lifecycle error model does not provide:

* Component-level error inspection.
* Error aggregation APIs.
* Error hierarchies.
* Remediation frameworks.
* Retry frameworks.
* Failure classification systems.

The lifecycle manager reports lifecycle outcomes.

Observability systems report lifecycle details.
