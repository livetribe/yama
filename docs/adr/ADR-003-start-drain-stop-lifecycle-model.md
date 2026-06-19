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

type Drainer interface {
    Drain(context.Context) error
}

type Stopper interface {
    Stop(context.Context) error
}
```

Lifecycle capabilities are optional.

A component may implement:

* None
* Starter only
* Drainer only
* Stopper only
* Any combination of the three

The framework shall not support additional lifecycle phases.

## Public API

The lifecycle manager shall expose:

```go
type Lifecycle interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

Applications shall not invoke Drain directly.

Drain is an internal lifecycle phase managed by the framework.

The framework guarantees that whenever shutdown occurs:

```text
Drain
  ↓
Stop
```

is executed for participating components.

This guarantee applies regardless of the reason shutdown was initiated.

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
KafkaConsumer    Starter, Drainer, Stopper
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

Once the active startup level has quiesced, the lifecycle manager determines which components successfully started.

The lifecycle manager then automatically initiates shutdown processing for those successfully started components.

The application is not required to invoke Stop() after a failed Start().

### Startup Failure Recovery

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
Return startup error
```

The lifecycle manager shall perform best-effort cleanup of successfully started components before returning the startup failure.

The shutdown sequence used during startup failure is identical to the shutdown sequence used during normal operation.

No special-case shutdown path exists.

## Drain Semantics

Drain is a pre-shutdown notification phase.

The framework intentionally does not assign domain-specific meaning to Drain.

The framework guarantees only:

```text
Drain occurs before Stop.
```

The meaning of Drain is owned by the component implementation.

Examples of component-specific Drain behavior may include:

* Stop accepting requests.
* Stop consuming messages.
* Stop dequeuing work.
* Notify peers.
* Flush buffers.
* Publish state.
* Prepare resources for shutdown.

These behaviors are implementation details of the component rather than responsibilities of the lifecycle framework.

### Drain Execution

All Drainer implementations execute concurrently.

Drain does not use dependency ordering.

Example:

```text
Database
   ↓
Router
```

Both components may drain concurrently.

Dependency relationships do not influence drain execution.

### Drain Failure

Drain is best effort.

Failures do not prevent other drain operations from executing.

Drain failures do not prevent Stop execution.

Timeouts are treated as drain failures.

Drain failures contribute to overall shutdown failure semantics.

Drain failures do not alter shutdown execution.

The lifecycle manager continues into Stop processing and applies normal shutdown error semantics.

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

### Stop Failure

Stop is best effort.

Failures do not prevent remaining stop operations from executing.

Timeouts are treated as stop failures.

The lifecycle manager shall continue attempting shutdown even after one or more stop failures occur.

## Lifecycle Invariants

The framework guarantees the following invariants.

### Invariant 1

A dependency is started before any dependent that requires it.

### Invariant 2

A dependent is stopped before any dependency it requires.

### Invariant 3

If a component implements Drainer and Stopper:

```text
Drain
  ↓
Stop
```

always occurs in that order.

### Invariant 4

The same shutdown sequence is used for:

* Normal shutdown
* Startup failure recovery

### Invariant 5

Drain and Stop are best-effort operations.

Failures do not terminate the overall shutdown process.

### Invariant 6

The framework supports exactly three lifecycle phases:

* Start
* Drain
* Stop

No additional phases shall be introduced.

## Rationale

### Minimal Surface Area

Three phases are sufficient to express startup and graceful shutdown for the vast majority of applications.

Additional phases increase complexity without providing proportional value.

### Consistent Shutdown Behavior

Using the same Drain → Stop sequence for all shutdown scenarios reduces special-case behavior and simplifies component implementation.

### Strong Separation of Concerns

The framework defines lifecycle ordering.

Components define lifecycle behavior.

The framework does not attempt to interpret the meaning of Drain or Stop.

### Deterministic Generation

Three fixed phases allow execution ordering to be generated statically and reasoned about mechanically from the dependency graph.

## Rejected Alternatives

### Start and Stop Only

Rejected because many applications require a distinct pre-shutdown phase before final cleanup.

### Public Drain API

Rejected because applications could accidentally bypass Drain during shutdown.

The framework should guarantee correct lifecycle sequencing.

### Additional Lifecycle Phases

Rejected because they transform the lifecycle system into a generalized orchestration framework.

The project intentionally limits scope to startup and shutdown orchestration.

### Dependency-Ordered Drain

Rejected because Drain is a notification phase rather than a dependency-sensitive cleanup phase.

Dependency ordering is reserved for Start and Stop.

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
