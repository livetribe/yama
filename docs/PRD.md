# Product Requirements Document (PRD)

# Yama: Compile-Time Lifecycle Orchestration for Go Applications

## 1. Vision

Yama is a compile-time lifecycle orchestration framework for Go applications.

Yama derives lifecycle orchestration directly from a Google Wire dependency graph and generates lifecycle execution code during build time.

The project eliminates duplication between:

1. Dependency construction.
2. Startup orchestration.
3. Shutdown orchestration.

Yama follows the same core philosophy as Google Wire:

* Generate code.
* Avoid reflection.
* Avoid runtime registration.
* Avoid service locators.
* Avoid runtime graph construction.
* Preserve strong typing.
* Generate understandable Go code.

Yama is intentionally not a dependency injection framework.

Google Wire remains responsible for dependency construction.

Yama is responsible only for lifecycle orchestration.

---

# 2. Problem Statement

Dependency-injected applications frequently maintain the same dependency graph multiple times.

Example:

```text
Dependency Graph
    ↓
Dependency Construction

Dependency Graph
    ↓
Startup Ordering

Dependency Graph
    ↓
Shutdown Ordering
```

This duplication creates:

* Maintenance burden.
* Lifecycle drift.
* Startup ordering mistakes.
* Shutdown ordering mistakes.
* Reduced operational confidence.

The dependency graph already contains sufficient information to derive lifecycle ordering.

Yama eliminates duplicated lifecycle definitions by generating lifecycle orchestration directly from the Wire dependency graph.

---

# 3. Goals

## Primary Goals

### G1. Single Source of Truth

Use the Google Wire dependency graph as the authoritative source of lifecycle ordering.

### G2. Compile-Time Lifecycle Generation

Generate lifecycle orchestration during build time.

### G3. Strong Typing

Preserve compile-time type safety.

### G4. Deterministic Execution

Ensure lifecycle behavior is fully determined by generated code.

### G5. Readable Generated Code

Generate code that can be:

* Read.
* Reviewed.
* Debugged.
* Stepped through.

using ordinary Go tooling.

### G6. Minimal Public API

Expose only lifecycle concepts applications must directly interact with.

### G7. Extensibility

Provide interceptor-based lifecycle customization without expanding lifecycle manager responsibilities.

---

# 4. Non-Goals

Yama shall not provide:

## Dependency Injection

Google Wire remains responsible for dependency injection.

## Reflection

Runtime reflection is prohibited.

## Runtime Registration

Runtime registration systems are prohibited.

## Service Locators

Service locator APIs are prohibited.

## Runtime Graph Construction

Dependency graphs are generation-time artifacts.

## Runtime Lifecycle Plans

Lifecycle plans are not exposed at runtime.

## Runtime Graph Introspection

Graph inspection APIs are prohibited.

## Health Check Frameworks

Health management is outside project scope.

## Readiness Frameworks

Readiness management is outside project scope.

## Configuration Frameworks

Configuration acquisition, parsing, validation, and reload are outside project scope.

## Workflow Orchestration

Yama is not a workflow engine.

## Additional Lifecycle Phases

The framework supports exactly:

* Start
* Drain
* Stop

No additional phases shall be introduced.

---

# 5. Users

Primary users include:

* Go infrastructure engineers.
* Platform engineers.
* Backend engineers.
* Teams already using Google Wire.

Applications that do not use Google Wire are outside the primary target audience.

---

# 6. Functional Requirements

## 6.1 Lifecycle Participation

The framework shall support optional lifecycle capabilities.

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

Components may implement:

* None
* Starter
* Drainer
* Stopper
* Any combination

All Wire graph nodes participate in dependency analysis.

Only lifecycle-capable nodes participate in lifecycle execution.

---

## 6.2 Startup Behavior

Startup shall follow dependency ordering.

A component shall not start before its dependencies have started successfully.

Independent dependency branches shall start concurrently.

Startup shall fail fast.

Startup failure shall:

1. Cancel startup execution.
2. Initiate shutdown cleanup.
3. Return a lifecycle startup failure.

---

## 6.3 Drain Behavior

Drain is a pre-shutdown notification phase.

Drain shall:

* Execute concurrently.
* Ignore dependency ordering.
* Execute before Stop.

Drain failures shall not prevent Stop execution.

Applications shall not invoke Drain directly.

Drain is managed internally by the lifecycle manager.

---

## 6.4 Shutdown Behavior

Shutdown shall execute in reverse dependency order.

A dependent shall stop before the dependency it relies upon.

Independent branches shall stop concurrently.

Shutdown shall be best effort.

Failures shall not prevent remaining shutdown work.

---

## 6.5 Startup Failure Cleanup

If startup fails after one or more components have started:

```text
Start
  ↓
Failure
  ↓
Drain
  ↓
Stop
```

The same shutdown sequence used during normal operation shall be used during startup failure cleanup.

---

## 6.6 Lifecycle Configuration

The framework shall support strongly-typed lifecycle configuration.

Lifecycle configuration currently supports operation-specific timeouts:

* Start timeout.
* Drain timeout.
* Stop timeout.

Configuration shall be generated as application-specific generated structures.

The framework consumes configuration but does not provide:

* Configuration loading.
* Configuration parsing.
* Configuration discovery.
* Configuration validation frameworks.
* Configuration precedence systems.
* Runtime configuration reload.

Applications remain responsible for constructing and supplying lifecycle configuration.

Generated lifecycle configuration structures are implementation artifacts and are not part of the lifecycle library's stable public API.

---

## 6.7 Interceptors

The framework shall support lifecycle interceptors.

Interceptors shall be:

* Runtime objects.
* Operation-specific.
* Strongly typed.

Separate interceptor chains shall exist for:

* Start
* Drain
* Stop

Interceptors may:

* Observe behavior.
* Modify behavior.
* Modify context.
* Suppress execution.
* Modify lifecycle outcomes.

Interceptors may be attached:

* Globally.
* Per lifecycle participant.

---

## 6.8 Lifecycle Metadata

The lifecycle manager shall provide lifecycle metadata through context propagation.

Metadata shall support:

* Logging.
* Metrics.
* Telemetry.
* Tracing.

Metadata shall include the lifecycle participant identity being started, drained, or stopped.

The exact metadata representation is an implementation detail.

---

## 6.9 Error Handling

The framework shall expose lifecycle-level errors only.

The framework shall not expose:

* Component errors.
* Component identity.
* Component diagnostics.
* Error aggregation structures.

Detailed diagnostics shall be provided through interceptors and observability integrations.

---

## 6.10 Code Generation

The framework shall generate lifecycle orchestration code during build time.

Generated code shall:

* Compile without reflection.
* Be understandable by humans.
* Be debuggable.
* Be deterministic.

Generated code shall be executable Go code rather than runtime lifecycle plans.

---

# 7. Non-Functional Requirements

## 7.1 Type Safety

The framework shall preserve compile-time type safety.

## 7.2 Determinism

Lifecycle execution shall be deterministic.

## 7.3 Debuggability

Generated code shall be navigable using standard IDE tooling.

## 7.4 Maintainability

Generated code shall prioritize readability over code generation cleverness.

## 7.5 Performance

Lifecycle analysis shall occur during generation rather than execution.

Runtime overhead shall be minimized.

## 7.6 Observability

Lifecycle execution shall support comprehensive observability through interceptors.

---

# 8. Lifecycle Model

The framework supports exactly three lifecycle phases:

```text
Start
Drain
Stop
```

Applications interact with:

```text
Start
Stop
```

Drain remains an internal lifecycle phase.

Lifecycle phases are fixed.

The framework shall not support lifecycle phase customization.

---

# 9. Interceptor Model

Interceptors are the primary runtime extension mechanism.

The lifecycle manager remains focused on orchestration.

Examples of interceptor responsibilities:

* Logging
* Metrics
* Telemetry
* Tracing
* Diagnostics
* Conditional participation
* Environment-specific behavior

The lifecycle manager shall not absorb these concerns directly.

---

# 10. Error Model

The framework exposes lifecycle-level outcomes.

Public lifecycle errors include:

```text
ErrStartFailed
ErrStopFailed
```

Timeouts are treated as ordinary lifecycle failures.

The framework does not expose timeout-specific public errors.

Drain is an internal lifecycle phase.

The framework does not expose ErrDrainFailed.

Failures occurring during Drain contribute to shutdown failure semantics and may result in ErrStopFailed.

---

# 11. Code Generation Model

The framework shall:

1. Analyze the Wire graph.
2. Analyze lifecycle participation.
3. Analyze concurrency opportunities.
4. Generate lifecycle orchestration code.

The framework shall not generate:

* Runtime plans.
* Runtime graphs.
* Runtime graph interpreters.

Generated code is the authoritative lifecycle representation.

---

# 12. DAG Analysis Requirements

The framework shall analyze the Wire dependency graph to compute:

* Startup ordering.
* Shutdown ordering.
* Concurrency opportunities.
* Dependency relationships.

Analysis occurs entirely during generation.

No DAG analysis occurs during runtime execution.

---

# 13. Observability Requirements

Lifecycle observability shall be implemented through interceptors.

The framework shall support observation of:

* Start execution.
* Drain execution.
* Stop execution.
* Failures.
* Timeouts.
* Duration measurements.

The framework shall remain observability-tool agnostic.

---

# 14. Generated Artifacts

Expected generated artifacts include:

```text
lifecycle_gen.go
```

and generated:

* Lifecycle implementations.
* Lifecycle configuration structures.
* Lifecycle orchestration code.
* Concurrency helper structures.
* Lifecycle chain construction code.

Generated artifacts are implementation details.

Generated artifacts are not part of the stable public API.

---

# 15. Acceptance Criteria

The project is considered successful when:

1. Lifecycle orchestration can be generated directly from a Wire graph.
2. Startup ordering is automatically derived.
3. Shutdown ordering is automatically derived.
4. Independent branches execute concurrently.
5. No reflection is required.
6. No runtime graph construction is required.
7. Generated code remains readable.
8. Generated code remains debuggable.
9. Interceptors provide lifecycle extensibility.
10. Lifecycle configuration remains strongly typed.

---

# 16. Risks

## Scope Expansion

The project may accumulate unrelated framework responsibilities.

## Generated Code Complexity

Large dependency graphs may produce large generated artifacts.

## Interceptor Abuse

Interceptors may be used to implement unrelated concerns.

## Wire Coupling

The project intentionally depends on Wire concepts and workflows.

---

# 17. Open Questions

The following questions remain architectural rather than product-level concerns:

* Generated package layout.
* Generated naming conventions.
* Interceptor interface signatures.
* Context metadata representation.
* Chain construction implementation.

These questions shall be resolved during architecture design.

---

# 18. Future Enhancements

Potential future enhancements include:

* Additional lifecycle configuration options.
* Lifecycle configuration tooling.
* Additional observability integrations.
* Optional configuration binding helpers.
* Lifecycle visualization tooling.

Future enhancements must preserve:

* Compile-time operation.
* Strong typing.
* Minimal public API.
* Generated-code-first philosophy.
