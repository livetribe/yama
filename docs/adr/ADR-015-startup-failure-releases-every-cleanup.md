# ADR-015: A Startup Failure Releases Every Cleanup

## Status

Accepted

## Context

The generated constructor builds every component before the application
calls `Start`. Each provider acquires its resources when it runs. A cleanup
releases what its provider acquired. The whole graph therefore holds its
construction resources before startup begins. A startup failure does not
release them.

ADR-003 makes startup fail-fast. When a `Start` fails, the traversal stops
at the level that failed. The rollback then quiesces and tears down the
reached levels only. A cleanup in a level after the failed level does not
run. The resources that its provider acquired stay held.

Inside a reached level, the rule is different. A cleanup runs there even
when its own component's `Start` failed. Yama gates a failed component out
of the quiesce and teardown passes, but the gates do not apply to cleanups.
The same provider's cleanup therefore runs in the failed level and does not
run one level later.

Google Wire's own contract has two parts. When a provider returns an error
during construction, the injector runs the cleanups of the values that it
already built. ADR-008 keeps that unwind through re-emission. When
construction succeeds, Google Wire returns one aggregate cleanup function
to the caller. That function runs every cleanup. ADR-008 removes the
aggregate from the constructor's results and gives teardown to the
lifecycle.

Some cleanups release resources outside the process: a distributed lock, a
lease, a registration in service discovery, a temporary file. Process exit
does not release them. An operator usually retries after a startup failure.
A resource that stays held can block that retry.

## Decision

### Every teardown runs every cleanup

When the lifecycle tears the graph down, it runs every cleanup. The set of
cleanups that run does not depend on how far startup reached. This rule
covers a normal `Stop` and the rollback after a failed `Start`.

### The teardown pass after a startup failure walks every level

When a `Start` fails, the quiesce pass covers the reached levels only. A
component in an unreached level has no work in flight, so there is nothing
to quiesce. The teardown pass then walks every level in reverse order, from
the last level down to the first.

In an unreached level, only the cleanups run. A component in that level
receives no lifecycle call. In a reached level, nothing changes. The gates
keep a failed component out of the capability passes, and the cleanups run
without a gate.

### Nothing else changes

A normal `Stop` is unchanged. When a provider fails during construction,
Google Wire unwinds the values that it built. That unwind is unchanged. The
order between a cleanup and its own component's `Stop` is unchanged.

## Rationale

### The gate for a cleanup is completed construction, not a start outcome

The gates exist because `Stop` undoes what `Start` did. A component that
did not start did no start work, so the gates correctly keep it out of the
capability passes. A cleanup undoes what the provider did. Every
component's provider already ran before the application called `Start`.
The gate for a cleanup is therefore completed construction. That condition
became true when the constructor returned.

The reached-level rule already applies this principle. A cleanup runs in a
reached level even when its own component's `Start` failed. This decision
applies the same principle across levels.

### The lifecycle took ownership of the aggregate cleanup's work

A Google Wire caller deferred the aggregate cleanup function. That call ran
every cleanup on every exit path. The progress of the application's own
startup logic did not change that. ADR-008 removes the aggregate and gives
teardown to the lifecycle.

ADR-003 states that the application is not required to call `Stop` after a
failed `Start`, and ADR-006 makes that call a no-op. The rollback is
therefore the only code path that can run the cleanups after a startup
failure. When the rollback runs a subset of the cleanups, no code path runs
the rest. One exit path stays uncovered: a lifecycle that the application
constructed and never started. The Non-Goals section records that path.

### The application cannot predict a component's level

Yama computes the levels from the dependency graph. The application author
does not select them. An edit elsewhere in the graph can move a component
to a different level. If a cleanup runs only in reached levels, a graph
edit far from the component decides whether its resources leak. An author
cannot rely on a rule that depends on a computed position.

## Consequences

### Positive

When a level's start fails, every cleanup runs before `Start` returns. The
teardown rule
becomes uniform. Teardown releases what construction and startup acquired.
`Stop` and `Quiesce` run where `Start` succeeded. Cleanups run in every
level, because construction succeeded in every level.

The reached-level rule and the cross-level rule now state one principle. A
reader who knows the reached-level rule can predict what happens one level
later.

### Negative

The teardown pass gains a second mode. In a reached level it runs gated
component teardown and cleanups. In an unreached level, it runs only
cleanups. The runtime-support package must hold that distinction.

ADR-003 stated that a component in an unreached level takes no part in
shutdown. That statement stays true for the capability passes and became
false for cleanups. ADR-003 now states the narrowed rule.

### Accepted Trade-Off

A cleanup can now run for a component that never started. A cleanup that
assumes a completed `Start` observes an unexpected state. Two reasons make
the exposure acceptable. Google Wire has no start phase, so a
cleanup written for Google Wire cannot assume one. And the exposure already
exists inside every reached level, where a cleanup runs after its own
component's `Start` failed.

## Rejected Alternatives

### Keep the traversal and document the leak

The code stays as it is, and the documentation records that a cleanup after
the failed level does not run. Rejected, for the three reasons that the
Rationale gives. The author cannot predict a component's level. The reached
levels follow a different rule. No code path runs the rest of the aggregate
cleanup's work.

### Run the cleanups of the unreached levels before the quiesce pass

The rollback runs the cleanups of the unreached levels first, before it
quiesces the reached levels. The reached components then still accept new
work while the cleanups run. Rejected.

In a normal `Stop`, every cleanup runs in the teardown pass, after every
quiesce completes. After a quiesce, a dependency can refuse new work. A
cleanup that sends new work to a dependency meets that refusal in a normal
`Stop` as well. The failure path must not give a cleanup an earlier
position than the normal path gives it. Every quiesce still completes
before any teardown begins.

## Non-Goals

This ADR does not cover a lifecycle that the application constructed and
never started. `Stop` on that lifecycle is a no-op, so no code path runs
its cleanups. A `Start` under a context that is already done leaves the
lifecycle in that same state. That call runs no level and no cleanup. It
returns `ErrStartFailed` and leaves the lifecycle startable. This ADR
records that gap and does not close it.

This ADR does not change interception. A cleanup passes through no
interceptor chain, as ADR-008 states. A component in an unreached level
receives no lifecycle call. No gate runs for it, and Yama logs nothing for
it.

This ADR does not change how a component's own cleanup and `Stop` relate.
ADR-008 fixes that order.
