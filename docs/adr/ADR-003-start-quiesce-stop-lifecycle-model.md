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
interfaces above:

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

All components in the Wire dependency graph participate in dependency analysis.

A component takes a place in lifecycle execution when it implements one or more
lifecycle capability interfaces, or when its provider returned a Google Wire
cleanup function. The second case is what gives a cleanup its position in the
ordering; such a component receives no lifecycle callback, because it
implements no capability to call — only its cleanup runs.

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

A component with neither a capability nor a cleanup appears in no level. It still
influences ordering: a component that reaches another only through it is still
ordered after that one.

Dependency-only components receive no lifecycle callbacks.

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

1. Additional startup work shall not be scheduled; no later level is started.
2. In-flight startup operations in the active level are not canceled.
3. Startup is considered failed.

Siblings within a level are independent subsystems, not collaborating halves of one
job, so a failure in one is not a reason to interrupt the others. Cancelling them
would also leave a component interrupted part-way through `Start` — a state it must
unwind itself, since a failed `Start` receives no teardown. Letting each sibling
finish on its own terms means every one ends either started, and therefore torn down
cleanly, or failed, and therefore responsible for itself. The framework cancels
nothing of its own; components observe the caller's context, whose deadline still
bounds a slow `Start`.

The lifecycle manager waits for in-flight startup operations in the active startup level to complete.

Once the active startup level has settled, the lifecycle manager determines which components successfully started.

A component that implements `Starter` counts as started only if its `Start` returned
without error or panic. A component that does not implement `Starter` has no start to succeed
or fail; it counts as started if, and only if, its level was reached during startup — the
traversal advanced to it before failing. Reaching a component's level is that component's
start. Consequently, the lifecycle manager does not bring up a non-`Starter` in a level
that the failed startup never reached. That component takes no part in the quiesce and
teardown capability passes. A `Starter` in an unreached level takes no part in them
either. A Google Wire cleanup in an unreached level still runs during the teardown pass
(ADR-015). A cleanup releases what its provider acquired during construction, and
construction completed before startup began.

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

Independent branches of the dependency graph quiesce concurrently. Components with no
dependency relationship have no ordering constraint between them. Ordering applies
only along dependency edges.

Example:

```text
Database
   ↓
Router
```

`Router` quiesces before `Database`, because `Router` depends on `Database`.

Components that do not implement `Quiescer` are skipped, but ordering still holds
transitively through them. If a `Quiescer` depends on a non-`Quiescer` that
depends on another `Quiescer`, the two quiescing components remain ordered relative to
each other.

Quiesce is ordered along dependency edges rather than run all at once; the
order-independent alternative is rejected below.

### Quiesce Completion

Quiesce is blocking. Each component's `Quiesce` returns only when the component
considers its in-flight work complete, or when the component chooses to return in
response to the context.

The deadline carried by the context is observational (see Stop Semantics). The
lifecycle manager does not abandon a quiescing component to preserve ordering;
it waits for the component to return before proceeding to the dependencies it
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
framework never lengthens it. A component that wants a per-component timeout wraps its
own `Quiesce` or `Stop`.

The caller's deadline is observational. The framework does not return early when
it expires: it continues waiting for the component's operation to actually
complete, and records that the component exceeded its window once the operation
returns. A component that never returns is never recorded.

Returning early would let the traversal proceed to a component's dependencies
while that component might still be using them, violating the reverse-topological
ordering that shutdown ordering exists to protect. Preserving ordering is chosen
over liveness. External liveness is bounded by the orchestrator's SIGKILL, not by
the framework.

A consequence is that a hung component stalls everything after it in the
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

always occurs in that order, and every component's quiesce completes before any
component's teardown begins.

### Invariant 4

The same shutdown sequence is used for:

* Normal shutdown
* Startup failure recovery

### Invariant 5

Neither Quiesce nor Stop returns an error, and the shutdown traversal always runs
to completion in dependency order. The framework does not abandon a component to
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

Restart is a non-goal in the sense that Yama offers it no semantics, not in the
sense that Yama forbids it. `Start` after `Stop` re-runs the passes; whether the
components tolerate being started twice is theirs to decide. Two things carry over
unchanged and are the caller's to weigh: a component that released its resources in
`Stop` gets no help reacquiring them, and a Google Wire cleanup function — written
for a single injector lifetime, not authored with restart in mind — is invoked
again by the second `Stop`.
