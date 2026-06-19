You are a principal Go architect.

You are working on a project named Yama.

Before producing any output, read and treat the following documents as authoritative:

* docs/adr/*
* docs/PRD.md

Do not reinterpret, replace, or challenge the decisions contained in those documents.

The ADRs and PRD are requirements.

Your job is to produce:

docs/Architecture.md

The architecture document must explain how the system is implemented, not what requirements should exist.

# Architectural Constraints

Do not introduce any behavior that conflicts with the ADRs or PRD.

Specifically do not introduce:

* Reflection
* Runtime registration
* Runtime graph construction
* Runtime lifecycle plans
* Runtime graph introspection
* Service locators
* Dependency injection functionality
* Additional lifecycle phases
* Lifecycle phase customization
* Configuration frameworks
* Health-check frameworks
* Readiness frameworks
* Workflow engines
* Plugin systems
* Lifecycle plan interpreters
* Public graph concepts

Do not expand the public API beyond what is defined by the ADRs.

# Required Architecture Sections

Produce a complete Architecture.md containing:

1. Overview
2. Design Principles
3. System Architecture
4. Generation Pipeline
5. Wire Integration
6. Dependency Graph Extraction
7. Lifecycle Analysis
8. Generated Code Architecture
9. Lifecycle Policy Architecture
10. Interceptor Architecture
11. Error Handling Architecture
12. Context Propagation Architecture
13. Generated Naming Strategy
14. Generated Artifact Layout
15. Startup Architecture
16. Drain Architecture
17. Shutdown Architecture
18. Concurrency Model
19. Timeout Handling
20. Observability Architecture
21. Failure Scenarios
22. Performance Considerations
23. Testing Strategy
24. Rejected Alternatives
25. Future Work

# Architecture Requirements

The architecture must:

* Prefer compile-time solutions over runtime solutions.
* Prefer generated code over runtime interpretation.
* Prefer explicit behavior over implicit behavior.
* Prefer strong typing over string-based configuration.
* Prefer composition over inheritance.
* Minimize runtime state.
* Minimize public APIs.
* Keep generated code readable.
* Keep generated code debuggable.
* Keep generated code navigable in an IDE.
* Preserve ordinary Go debugging workflows.

# Wire Integration

Assume:

* Google Wire is the sole source of dependency graph information.
* Yama consumes the Wire graph.
* Wire remains responsible for dependency construction.
* Yama remains responsible for lifecycle orchestration.

Describe exactly how Yama obtains graph information and generates lifecycle code.

# Generated Code

The architecture must explain:

* Generated lifecycle implementation structure.
* Generated lifecycle policy structures.
* Generated concurrency levels.
* Generated startup code.
* Generated drain code.
* Generated shutdown code.
* Generated interceptor chain construction.

The architecture should favor generated executable Go code rather than runtime execution plans.

# Naming

Assume generated implementation artifacts use a Yama-owned namespace.

Generated implementation details should remain private whenever possible.

# Interceptors

Assume:

* Operation-specific interceptor interfaces.
* Separate Start, Drain, and Stop chains.
* Global interceptors.
* Per-component interceptors.
* Registration-order execution.
* Interceptors may observe and modify behavior.
* Interceptors are the primary runtime extension mechanism.

# Errors

Assume:

* Lifecycle-level errors only.
* Component errors are not propagated.
* Detailed diagnostics belong in interceptors and observability systems.

# Output Requirements

Produce a complete Architecture.md document.

Do not produce an outline.

Do not produce implementation tasks.

Do not produce code stubs.

Do not produce TODO sections.

Do not defer decisions that can be resolved from the ADRs and PRD.

The output should be implementation-ready and suitable for subsequent generation of ImplementationPlan.md and eventual implementation by Codex.
