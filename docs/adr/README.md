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
| [ADR-009](ADR-009-boundary-lifecycle-nodes.md) | Boundary Lifecycle Nodes | Accepted |
| [ADR-010](ADR-010-runtime-support-package.md) | Runtime-Support Package and the Generated/Shared Split | Accepted |

## Conventions

New ADRs should:

* Use the filename format `ADR-NNN-short-title.md`.
* Use a monotonically increasing three-digit number.
* Include a status section near the top of the document.
* Record the decision, rationale, consequences, and rejected alternatives.
* Prefer amending or superseding an existing ADR over rewriting history.

Accepted ADRs represent the current architectural direction unless a later ADR
explicitly supersedes them.
