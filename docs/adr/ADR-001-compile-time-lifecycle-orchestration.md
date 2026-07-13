# ADR-001: Compile-Time Lifecycle Orchestration

## Status

Accepted

## Context

Dependency-injected Go applications frequently maintain the same dependency graph in multiple places:

1. Dependency construction.
2. Startup ordering.
3. Shutdown ordering.

Dependency construction is often handled through compile-time dependency injection tools such as Google Wire. However, lifecycle orchestration is typically implemented separately using one or more of the following approaches:

* Hand-written startup and shutdown sequences.
* Runtime registration systems.
* Reflection-based lifecycle frameworks.
* Application frameworks that own process startup and shutdown.

These approaches introduce duplication between the dependency graph and the lifecycle graph.

As applications evolve, the dependency graph and lifecycle graph can drift apart. Components may be started in the wrong order, shut down in the wrong order, or omitted from lifecycle management entirely.

The project requires a mechanism that derives lifecycle orchestration directly from the existing dependency graph while preserving Go's static typing and explicitness.

## Decision

The framework shall implement lifecycle orchestration through compile-time code generation.

The generator shall consume dependency graph information and emit lifecycle orchestration code during the build process.

Generated code shall contain:

* Startup orchestration.
* Quiesce orchestration.
* Shutdown orchestration.
* Lifecycle implementation code.

The generated code shall be ordinary Go code that can be read, debugged, stepped through, and reviewed by application developers.

No lifecycle graph construction shall occur at runtime.

No runtime registration shall be required.

No reflection shall be used for orchestration.

## Rationale

### Single Source of Truth

The dependency graph already describes component relationships.

A component that depends on another component cannot safely start before that dependency is available.

Likewise, a component should generally stop before the dependencies it relies upon are removed.

The dependency graph already contains the information required to derive lifecycle ordering.

Generating orchestration from that graph eliminates duplicated lifecycle definitions.

### Static Verification

Compile-time generation allows lifecycle planning errors to be detected before deployment.

Failures become build-time failures rather than runtime failures.

This aligns with the project's preference for deterministic behavior and explicit correctness.

### Strong Typing

Generated code preserves Go's type system.

Applications interact with concrete types and interfaces rather than string identifiers, registries, metadata objects, or reflection-based abstractions.

The compiler remains the primary validation mechanism.

### Operational Simplicity

Generated orchestration logic becomes part of the application binary.

No runtime graph builder, registry, planner, dependency resolver, or lifecycle engine must execute during startup.

Runtime behavior is reduced to executing generated code.

### Debuggability

Generated Go code can be:

* Reviewed in code review.
* Inspected during debugging.
* Stepped through with standard Go tooling.
* Profiled using ordinary profiling tools.

The orchestration behavior remains visible rather than hidden inside framework internals.

### Alignment with Go Ecosystem Practices

This approach follows the philosophy established by Google Wire:

* Explicit dependencies.
* Compile-time generation.
* Minimal runtime machinery.
* Readable generated code.

Developers familiar with Wire should find the lifecycle framework conceptually consistent.

## Consequences

### Positive

* Eliminates duplication between dependency and lifecycle graphs.
* No reflection overhead.
* No runtime registration.
* No service locator patterns.
* Deterministic lifecycle execution.
* Strong compile-time validation.
* Readable generated artifacts.
* Small runtime footprint.
* Easier operational debugging.

### Negative

* Requires code generation as part of the build process.
* Lifecycle changes require regeneration.
* Generated files become part of the repository workflow.
* More complex build tooling than purely handwritten lifecycle code.

### Accepted Trade-Off

The project intentionally accepts additional build-time complexity in exchange for:

* Reduced runtime complexity.
* Improved determinism.
* Better type safety.
* Improved maintainability.

## Rejected Alternatives

### Hand-Written Lifecycle Management

Rejected because lifecycle ordering duplicates information already present in the dependency graph.

The duplication creates long-term maintenance burden and opportunities for drift.

### Runtime Registration Systems

Rejected because they require applications to maintain an additional lifecycle graph through registration calls.

Examples include systems where components register themselves during startup.

These systems introduce additional sources of truth and runtime validation requirements.

### Reflection-Based Lifecycle Frameworks

Rejected because they:

* Reduce type safety.
* Hide behavior behind framework internals.
* Increase runtime complexity.
* Make orchestration harder to understand and debug.

### Application Framework Ownership

Rejected because the project is intended to be a focused lifecycle library rather than an application framework.

The framework should orchestrate lifecycle behavior without owning the application runtime.

### Service Locator Architectures

Rejected because service locators obscure dependency relationships and move dependency validation from compile time to runtime.

## Diagnostics and Error Reporting

The framework reports lifecycle outcomes.

Detailed diagnostics are intentionally separated from lifecycle orchestration.

Examples of diagnostic information include:

* Component identity.
* Operation identity.
* Original component errors.
* Timeout information.
* Execution duration.

Such information belongs in interceptors and observability integrations.

The lifecycle manager itself exposes only lifecycle-level outcomes.

## Non-Goals

This decision does not imply:

* Dependency injection functionality.
* Runtime dependency resolution.
* Runtime graph introspection.
* Framework-managed remediation.
* Framework-managed retries.
* Framework-managed backoff.
* Generic workflow orchestration.

The generated lifecycle implementation exists solely to orchestrate Start, Quiesce, and Stop operations derived from a dependency graph.
