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
* Quiesce
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

type Quiescer interface {
    Quiesce(context.Context)
}

type Stopper interface {
    Stop(context.Context)
}
```

`Start` returns an error; `Quiesce` and `Stop` do not. The signature of each
interface matches the error semantics of the phase it represents.

Components may implement:

* None
* Starter
* Quiescer
* Stopper
* Any combination

All Wire graph components participate in dependency analysis.

Only lifecycle-capable components participate in lifecycle execution.

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

## 6.3 Quiesce Behavior

Quiesce is the first pass of shutdown. A component that quiesces stops accepting
new work and then blocks until its in-flight work completes, subject to context.

Quiesce shall:

* Execute in reverse dependency order (dependents before dependencies), the same
  direction as Stop.
* Execute independent branches concurrently.
* Complete before teardown begins.

`Quiescer.Quiesce` returns no error.

Applications shall not invoke Quiesce directly. It is not exposed on `Lifecycle`.
`Stop` runs the quiesce pass internally as its unconditional first action.

---

## 6.4 Shutdown Behavior

Shutdown shall execute in reverse dependency order.

A dependent shall stop before the dependency it relies upon.

Independent branches shall stop concurrently.

Shutdown shall run to completion in dependency order. The caller's context
deadline is observational: when it fires the framework logs per-component overrun but
does not return early, so a hung component stalls everything after it until
SIGKILL. Preserving ordering is chosen over liveness.

---

## 6.5 Startup Failure Cleanup

If startup fails after one or more components have started:

```text
Start
  ↓
Failure
  ↓
Stop (quiesce pass, then teardown)
```

The same shutdown sequence used during normal operation shall be used during startup failure cleanup.

---

## 6.6 Lifecycle Configuration

The framework shall generate no lifecycle configuration.

There is no generated configuration structure and no start or shutdown deadline
field. The only deadline is the one carried by the context the caller passes to
`Start` and `Stop`. The framework threads that context through the traversal and
never lengthens its deadline. The deadline is observational: exceeding it is
logged, not enforced, and the traversal continues to completion.

A component that wants a per-component timeout wraps its own `Start`, `Quiesce`, or
`Stop`. This is ordinary Go, not a framework mechanism. Slow-operation and overrun
diagnostics are interceptor concerns.

The framework provides no configuration loading, parsing, discovery, validation,
precedence, or reload.

---

## 6.7 Interceptors

The framework shall support lifecycle interceptors.

Interceptors shall be:

* Runtime objects.
* Operation-specific.
* Strongly typed.
* Not uniform: each interceptor's signature matches the error semantics of the
  phase it wraps (Start propagates an error; Quiesce and Stop do not).

Separate interceptor chains shall exist for:

* Start
* Quiesce
* Stop

Interceptors may:

* Observe behavior.
* Modify behavior.
* Modify context.
* Suppress execution.
* Modify lifecycle outcomes.

Interceptors may be attached:

* Globally.
* Per lifecycle component, through generated, strongly-typed per-component
  fields (not string keys or a registration API).

---

## 6.8 Lifecycle Metadata

The lifecycle manager shall provide lifecycle metadata through context propagation.

Metadata shall support:

* Logging.
* Metrics.
* Telemetry.
* Tracing.

Metadata shall include the lifecycle component identity being started, quiesced, or stopped.

The exact metadata representation is an implementation detail.

Through interceptors, and remaining observability-tool agnostic, the framework shall support observation of start, quiesce, and stop execution, failures, deadline overruns, and durations.

---

## 6.9 Error Handling

The framework shall expose a single lifecycle-level error, `ErrStartFailed`.
`Start` is the only lifecycle operation that returns an error; `Stop` returns
nothing.

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

# 8. Acceptance Criteria

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
10. The public API stays minimal: capability interfaces, interceptor interfaces, the `Lifecycle` type, and `ErrStartFailed`.

---

# 9. Risks

## Scope Expansion

The project may accumulate unrelated framework responsibilities.

## Generated Code Complexity

Large dependency graphs may produce large generated artifacts.

## Interceptor Abuse

Interceptors may be used to implement unrelated concerns.

## Wire Coupling

The project intentionally depends on Wire concepts and workflows.

---

# 10. Future Enhancements

Potential future enhancements include:

* Additional observability integrations.
* Lifecycle visualization tooling.

Future enhancements must preserve:

* Compile-time operation.
* Strong typing.
* Minimal public API.
* Generated-code-first philosophy.
