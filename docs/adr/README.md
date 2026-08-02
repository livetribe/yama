# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for Yama.

ADRs document significant architectural decisions, the context that shaped them,
the alternatives considered, and the consequences accepted by the project. They
are intended to preserve design rationale for future maintainers and reviewers.

## Index

| ADR | Title | Status |
| --- | --- | --- |
| [ADR-001](ADR-001-compile-time-lifecycle-orchestration.md) | Compile-Time Lifecycle Orchestration | Accepted |
| [ADR-002](ADR-002-wire-as-graph-source.md) | Google Wire as the Authoritative Dependency Graph | Accepted |
| [ADR-003](ADR-003-start-quiesce-stop-lifecycle-model.md) | Three-Phase Lifecycle Model | Accepted |
| [ADR-004](ADR-004-generated-execution-plans.md) | Lifecycle Orchestration as Generated Code | Accepted |
| [ADR-005](ADR-005-lifecycle-interceptors.md) | Lifecycle Interceptors | Accepted |
| [ADR-006](ADR-006-lifecycle-error-handling.md) | Lifecycle Error Model | Accepted |
| [ADR-007](ADR-007-public-api.md) | Minimal Public API | Accepted |
| [ADR-008](ADR-008-parse-generated-wire-output.md) | Derive Lifecycle Ordering by Parsing Wire's Generated Injector | Accepted |
| [ADR-009](ADR-009-boundary-lifecycle-components.md) | Boundary Lifecycle Components | Accepted |
| [ADR-010](ADR-010-runtime-support-package.md) | Runtime-Support Package and the Generated/Shared Split | Accepted |
| [ADR-011](ADR-011-lifecycle-constructor-stubs.md) | Lifecycle Stubs Declare the Constructor and Its Providers | Proposed |
| [ADR-012](ADR-012-wire-gen-cli-parity.md) | The Yama Command Mirrors `wire gen` | Proposed |

## Conventions

New ADRs should:

* Use the filename format `ADR-NNN-short-title.md`.
* Use a monotonically increasing three-digit number.
* Include a status section near the top of the document.
* Record the decision, rationale, consequences, and rejected alternatives.
* Use shared terms as defined in `docs/adr/glossary.md` rather than restating a
  definition in ADR prose; propose a new term there instead of coining one
  locally.

An ADR starts life as `Proposed`, not `Accepted`. It moves to `Accepted` once a
prototype or spike has exercised the decision — a design that hasn't touched code
yet is not settled, however confident the reasoning. A `Proposed` ADR is expected
to change; an `Accepted` one is not, so promoting one before it has survived
contact with an implementation invites the rewrite this pre-1.0 stability rule
elsewhere presumes it avoided.

Accepted ADRs represent the current architectural direction unless a later ADR
explicitly supersedes them.

An ADR here describes the decision as it currently stands, not the sequence of
revisions that produced it. When a decision changes, its ADR is rewritten to
state the new decision and the reasoning for it, and the superseded design moves
to that ADR's Rejected Alternatives if the reasoning against it is worth keeping.
There are no amendment sections and no change logs; `git log` is the history.

This holds while the project is pre-1.0 and has no external consumers relying on
a decision's stability. Once one does, a change that breaks them wants a new ADR
that supersedes the old one by number, so the old decision stays readable at the
version it applied to.
