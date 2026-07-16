# ADR-003: Three-Phase Lifecycle Model

## Status

Accepted

## Context

The framework requires a lifecycle model that can be derived from a dependency graph and applied consistently across applications.

Many lifecycle systems evolve into generalized workflow engines with numerous phases such as:

* Initialize
* Warmup
* Ready
* Pause
* Resume
* Suspend
* Cleanup
* Restart
* Finalize

Each additional phase increases:

* API surface area
* Configuration complexity
* Execution complexity
* Documentation burden
* Long-term maintenance cost

The project requires a lifecycle model that is:

* Minimal
* Deterministic
* Dependency-aware
* Easy to understand
* Easy to generate
* Difficult to misuse

The lifecycle model must support graceful startup and graceful shutdown without becoming a generalized orchestration framework.

## Decision

The framework shall support exactly three lifecycle phases:

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

The error semantics of each interface match the error semantics of the phase it
represents. `Start` can fail, and a failed start means the application does not
run, so `Start` returns an error. `Quiesce` and `Stop` are shutdown operations
with nothing actionable to return to a caller, so neither returns an error.

Lifecycle capabilities are optional.

A component may implement:

* None
* Starter only
* Quiescer only
* Stopper only
* Any combination of the three

The framework shall not support additional lifecycle phases.

## Public API

The lifecycle manager is exposed as `Lifecycle`, composed of the capability
interfaces above (ADR-007 owns the public-API shape):

```go
type Lifecycle interface {
    Starter
    Stopper
}
```

`Quiesce` is not part of `Lifecycle`'s public API. There is no
`Lifecycle.Quiesce(...)` method.

Quiescing has no standalone utility. It is only ever a precursor to Stop, and
there is no "quiesced and staying that way" steady state worth exposing. A caller
who wants shutdown simply calls `Stop()`.

`Stop(ctx)` runs the quiesce pass internally, unconditionally, as its first
action, and then performs teardown:

```text
Quiesce
  ↓
Stop teardown
```

The caller does not observe the quiesce result, retry it, or reason about its
timing. This guarantee applies regardless of the reason shutdown was initiated.

## Lifecycle Participation

All nodes in the Wire dependency graph participate in dependency analysis.

Only nodes implementing one or more lifecycle capability interfaces participate in lifecycle execution.

Example:

```text
Config
  ↓
KafkaConfig
  ↓
KafkaConsumer
  ↓
Router
```

The complete graph contributes to dependency analysis.

However:

```text
Config           No lifecycle capability
KafkaConfig      No lifecycle capability
KafkaConsumer    Starter, Quiescer, Stopper
Router           Starter, Stopper
```

Only lifecycle participants appear in generated execution code.

Dependency-only nodes influence ordering but do not receive lifecycle callbacks.

## Startup Semantics

Startup follows dependency-directed ordering.

A component shall not be started before its dependencies have successfully started.

Independent branches may start concurrently.

Example:

```text
      Database
      /      \
     /        \
 Router      Worker
```

Generated startup ordering:

```text
Group 1:
  Database

Group 2:
  Router
  Worker
```

Components within the same dependency level may start concurrently.

### Startup Failure

Startup is fail-fast.

When a startup operation fails:

1. The startup context is canceled.
2. Additional startup work shall not be scheduled.
3. In-flight startup operations may observe cancellation through context propagation.
4. Startup is considered failed.

After startup context cancellation, the lifecycle manager waits for in-flight startup operations in the active startup level to complete.

Once the active startup level has settled, the lifecycle manager determines which components successfully started.

The lifecycle manager then automatically initiates shutdown processing for those successfully started components.

The application is not required to invoke Stop() after a failed Start().

### Startup Failure Recovery

If startup fails after one or more components have successfully started:

```text
Start
  ↓
Failure
  ↓
Stop (quiesce pass, then teardown)
  ↓
Return startup error
```

The lifecycle manager shall run the same internal shutdown sequence that `Stop`
performs — the quiesce pass followed by teardown — scoped to the successfully
started components, before returning the startup failure.

The shutdown sequence used during startup failure is identical to the shutdown sequence used during normal operation.

No special-case shutdown path exists.

## Quiesce Semantics

Quiesce is the first pass of shutdown. A component that quiesces stops accepting
new work and then blocks until its in-flight work completes, subject to the
context it is given.

The framework does not assign further domain-specific meaning to Quiesce. The
concrete meaning of "stop accepting new work" and "in-flight work" is owned by
the component implementation.

Examples of component-specific Quiesce behavior may include:

* Stop accepting requests, then wait for outstanding requests to finish.
* Stop consuming messages, then wait for in-flight handlers to complete.
* Stop dequeuing work, then finish the work already in progress.

`Quiescer.Quiesce` returns no error. Quiescing produces nothing actionable for a
caller, and there is no recovery from shutdown.

### Quiesce Execution

Quiesce executes in **reverse-topological order** — dependents quiesce before the
dependencies they rely on. This is the same direction as Stop, and for the same
reason: a dependency (for example, a logger or a connection pool) must not
quiesce while a dependent might still call into it.

Independent branches of the dependency graph quiesce concurrently. Nodes with no
dependency relationship have no ordering constraint between them. Ordering applies
only along dependency edges.

Example:

```text
Database
   ↓
Router
```

`Router` quiesces before `Database`, because `Router` depends on `Database`.

Nodes that do not implement `Quiescer` are skipped, but ordering still holds
transitively through them. If a `Quiescer` depends on a non-`Quiescer` that
depends on another `Quiescer`, the two quiescing nodes remain ordered relative to
each other.

This reverses the earlier concurrent, order-independent model: quiesce is now
ordered along dependency edges rather than run all at once.

### Quiesce Completion

Quiesce is blocking. Each participant's `Quiesce` returns only when the component
considers its in-flight work complete, or when the component chooses to return in
response to the context.

The deadline carried by the context is observational (see Stop Semantics). The
lifecycle manager does not abandon a quiescing participant to preserve ordering;
it waits for the participant to return before proceeding to the dependencies it
protects.

## Stop Semantics

Stop performs final shutdown and cleanup.

Stop follows reverse dependency ordering.

A component shall be stopped before the dependencies it relies upon are stopped.

Independent branches may stop concurrently.

Example:

```text
      Database
      /      \
     /        \
 Router      Worker
```

Generated shutdown ordering:

```text
Group 1:
  Router
  Worker

Group 2:
  Database
```

### Stop Deadline

The framework owns no deadline of its own and generates no lifecycle
configuration. The only deadline is the one carried by the caller's context passed
to `Stop`. The quiesce pass and the teardown pass share that one context, and the
framework never lengthens it. A participant that wants a per-node timeout wraps its
own `Quiesce` or `Stop`.

The caller's deadline is observational. When it fires, the framework records that
the participant exceeded its window but does not return early. It continues waiting
for the participant's operation to actually complete.

Returning early would let the traversal proceed to a participant's dependencies
while that participant might still be using them, violating the reverse-topological
ordering that shutdown ordering exists to protect. Preserving ordering is chosen
over liveness. External liveness is bounded by the orchestrator's SIGKILL, not by
the framework.

A consequence is that a hung participant stalls everything after it in the
traversal until SIGKILL. This is intentional and follows directly from "the
framework waits, and ordering is never violated."

## Lifecycle Invariants

The framework guarantees the following invariants.

### Invariant 1

A dependency is started before any dependent that requires it.

### Invariant 2

A dependent is stopped before any dependency it requires.

### Invariant 3

`Stop` performs a complete quiesce pass before any teardown begins. For a
component that implements `Quiescer` and `Stopper`:

```text
Quiesce
  ↓
Stop teardown
```

always occurs in that order, and every participant's quiesce completes before any
participant's teardown begins.

### Invariant 4

The same shutdown sequence is used for:

* Normal shutdown
* Startup failure recovery

### Invariant 5

Neither Quiesce nor Stop returns an error, and the shutdown traversal always runs
to completion in dependency order. The framework does not abandon a participant to
make progress; ordering is never violated to reclaim liveness.

### Invariant 6

The framework supports exactly three lifecycle phases:

* Start
* Quiesce
* Stop

No additional phases shall be introduced.

## Rationale

### Minimal Surface Area

Three phases are sufficient to express startup and graceful shutdown for the vast majority of applications.

Additional phases increase complexity without providing proportional value.

### Consistent Shutdown Behavior

Using the same Quiesce → Stop sequence for all shutdown scenarios reduces special-case behavior and simplifies component implementation.

### Strong Separation of Concerns

The framework defines lifecycle ordering.

Components define lifecycle behavior.

The framework does not attempt to interpret the meaning of Quiesce or Stop.

### Deterministic Generation

Three fixed phases allow execution ordering to be generated statically and reasoned about mechanically from the dependency graph.

## Rejected Alternatives

### Start and Stop Only

Rejected because many applications require a distinct pre-shutdown phase before final cleanup.

### Public Quiesce API

Rejected because quiescing has no standalone utility. It is only ever a precursor
to Stop, and there is no "quiesced and staying that way" steady state to expose.
Folding it into `Stop` guarantees correct sequencing and keeps the public surface
minimal.

### Additional Lifecycle Phases

Rejected because they transform the lifecycle system into a generalized orchestration framework.

The project intentionally limits scope to startup and shutdown orchestration.

### Concurrent, Order-Independent Quiesce

Rejected because a dependency must not quiesce while a dependent might still call
into it. Quiescing dependents before their dependencies requires reverse-topological
ordering, the same direction as Stop. An order-independent quiesce would let a
shared dependency stop accepting work while a dependent is still using it.

## Non-Goals

This lifecycle model does not provide:

* Workflow orchestration
* Job scheduling
* State machine execution
* Pause/resume semantics
* Restart semantics
* Framework-managed remediation
* Framework-managed retries
* Framework-managed backoff

The lifecycle model exists solely to coordinate startup and shutdown behavior derived from a dependency graph.
