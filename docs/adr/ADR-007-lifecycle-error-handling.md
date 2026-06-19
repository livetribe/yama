# ADR-007: Lifecycle Error Model

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

The public lifecycle error surface consists of:

```go
var ErrStartFailed = errors.New("lifecycle start failed")
var ErrStopFailed  = errors.New("lifecycle stop failed")
```

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

If shutdown encounters one or more failures:

```go
err := lifecycle.Stop(ctx)
```

returns:

```go
ErrStopFailed
```
Any failure occurring during Drain or Stop contributes to shutdown failure semantics.

If one or more Drain or Stop operations fail, the lifecycle manager returns ErrStopFailed after shutdown processing completes.

The caller is informed that shutdown did not complete successfully.

The caller is not informed which components failed.

The caller is not informed why those components failed.

Detailed diagnostics belong in observability systems.

## Drain Errors

Drain is an internal lifecycle phase.

Applications do not invoke Drain directly.

Drain does not have a public error surface.

Drain failures participate in overall shutdown processing.

If drain failures contribute to an unsuccessful shutdown outcome:

```go
ErrStopFailed
```

may be returned.

No public:

```go
ErrDrainFailed
```

exists.

## Timeout Errors

Timeouts are treated as ordinary lifecycle failures.

The lifecycle manager does not expose timeout-specific errors.

Examples:

```text
Component startup timeout
Component drain timeout
Component shutdown timeout
```

all participate in normal lifecycle error processing.

The lifecycle manager does not distinguish timeout failures from other lifecycle failures at the public API level.

## Startup Failure Cleanup

If startup fails after one or more components have successfully started:

```text
Start
  ↓
Failure
  ↓
Drain
  ↓
Stop
  ↓
ErrStartFailed
```

The lifecycle manager performs best-effort cleanup.

Cleanup failures do not alter the public error returned.

The caller receives:

```go
ErrStartFailed
```

because the lifecycle operation that failed was startup.

Detailed cleanup diagnostics belong in observability systems.

## Best-Effort Shutdown

Shutdown is best effort.

If one component fails during Drain or Stop:

* Remaining Drain operations continue.
* Remaining Stop operations continue.

The lifecycle manager attempts to complete as much shutdown work as possible.

After shutdown completes:

```go
ErrStopFailed
```

is returned if one or more shutdown failures occurred.

## Diagnostics

Detailed diagnostics are intentionally separated from lifecycle outcomes.

Examples of diagnostic information include:

* Component identity.
* Operation identity.
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
Did shutdown succeed?
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

Rejected because timeouts are ordinary lifecycle failures.

Timeout diagnostics belong in observability systems.

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
