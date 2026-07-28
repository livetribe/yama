# ADR-004: Lifecycle Orchestration as Generated Code

## Status

Accepted

## Context

The framework derives lifecycle behavior from a compile-time dependency graph.

After lifecycle analysis is complete, the framework must determine how orchestration is represented in generated output.

Many lifecycle systems generate a runtime representation of orchestration.

Examples include:

```go
type ExecutionPlan struct {
    StartupGroups  []Group
    ShutdownGroups []Group
}
```

A runtime engine then interprets the generated plan:

```go
engine.Execute(plan)
```

This approach introduces:

* Runtime orchestration engines.
* Runtime plan interpreters.
* Runtime execution metadata — a description of the graph, separate from the
  code that acts on it.
* Runtime lifecycle state derived from that description.
* Indirection between generated artifacts and execution behavior.

The project follows the same philosophy as Google Wire:

* Analyze at generation time.
* Generate executable code.
* Execute generated code.
* Minimize runtime machinery.

Generated lifecycle behavior should be understandable by reading generated source code.

Developers should be able to:

* Navigate orchestration using IDE tooling.
* Step through orchestration using a debugger.
* Understand execution ordering from generated code.
* Inspect lifecycle behavior without understanding framework internals.

## Decision

Lifecycle orchestration shall be represented as generated Go code.

The framework shall not generate:

* Runtime lifecycle plans.
* Runtime lifecycle graphs.
* Runtime execution metadata describing the graph.
* Runtime lifecycle interpreters.
* Runtime orchestration engines.

All lifecycle analysis shall occur during code generation.

Generated artifacts shall contain lifecycle orchestration directly.

Conceptually:

```text
Wire Source Inputs
    ↓
wire gen
    ↓
Generated wire_gen.go
    ↓
AST Analysis (Lifecycle Ordering)
    ↓
Generated Lifecycle Go Code
    ↓
Execution
```

The framework shall not follow the model:

```text
Wire Source Inputs
    ↓
wire gen
    ↓
Generated wire_gen.go
    ↓
AST Analysis (Lifecycle Ordering)
    ↓
Generated Runtime Plan
    ↓
Runtime Interpreter
    ↓
Execution
```

### The Ordered Level List

Generated code declares its levels, in dependency order, one call per level.
Something must hold those levels between the constructor that declares them and
the `Start` that runs them, so the lifecycle value holds them in an ordered list
and walks it forward to start and backward for the quiesce and teardown passes.

A list of levels held at runtime resembles the `Plan` above closely enough that
the difference is worth stating rather than assuming. What this decision rejects
is a *plan*: a description of execution, separate from the code that performs it,
that an engine reads. Three properties separate the two.

* **Nothing describes the execution apart from the code that performs it.** There
  is no serialized form, no schema, no plan type an application can hold or hand
  to an engine, and no representation of the graph — a level holds the callables
  it will invoke, not a description of them. The list is the direct in-memory
  consequence of running explicit declaration calls, in the order they appear in
  their source. A plan is *interpreted*; this list is only ever walked.
* **No lifecycle analysis happens at runtime.** Nothing sorts, traverses a graph,
  computes a topological order, or plans execution. Ordering is not derived from
  anything at runtime — it is the order the declaration calls arrived in. What
  classification remains is type assertion: each component and each supplied
  interceptor is asserted against the relevant interfaces once, at construction.
  That is Go's own mechanism for the question "does this value implement this
  interface," not a discovery pass over a graph.
* **Nothing can reorder a declared level.** The list is append-only, and position
  is fixed at the moment of declaration. No input to the runtime — no
  configuration, no registration, no data — moves a level relative to another.
  Changing the graph's execution order requires regenerating the constructor that
  declares it.

The graph's own levels come only from generated statements. Two levels do not:
the boundary sets registered through `WithBeginComponents` and
`WithEndComponents` (ADR-009) are declared by the runtime-support package as the
first and last levels. That is a call-site input, and it is worth being precise
about what it can and cannot do. It changes *what is in* the two extreme levels;
it cannot change where those levels sit, cannot reach the graph's levels, and
cannot reorder anything. A boundary component's position is granted by which
option registered it, not computed from it — which is the whole of ADR-009's
mechanism, and is why a boundary registration is a declaration in the same sense
a generated `NextLevel` call is.

The permission extends to an ordered list of declared levels and nothing further.
It does not extend to any runtime introspection API over that list, to computing
or amending ordering at runtime, or to a plan artifact in any form an
application, a tool, or a later run could read, write, or supply. A change that
would let two runs execute the same declarations in different orders crosses this
line and needs its own decision.

## Rationale

### Alignment with Google Wire

Google Wire performs dependency analysis during generation and emits executable Go code.

Yama extends the same philosophy to lifecycle orchestration.

The output should be executable code rather than a runtime representation of execution.

### Debuggability

Generated code should be inspectable using ordinary tooling.

Developers should be able to:

* Set breakpoints.
* Step through lifecycle execution.
* Navigate call hierarchies.
* Inspect orchestration logic directly.

No special lifecycle tooling should be required to understand application behavior.

### Reduced Runtime Complexity

By generating executable code directly, the framework avoids:

* Runtime graph traversal.
* Runtime topological sorting.
* Runtime execution planning.
* Runtime orchestration engines.

Runtime behavior is reduced to executing generated code.

### Deterministic Behavior

All lifecycle analysis occurs during generation.

Runtime behavior is therefore fully determined by generated source code.

Generated code becomes the authoritative description of lifecycle behavior.

## Lifecycle Analysis

Lifecycle analysis occurs entirely during code generation.

Analysis includes:

* Dependency ordering.
* Startup ordering.
* Shutdown ordering.
* Quiesce participation.
* Concurrency opportunities.
* Lifecycle capability detection.

No lifecycle analysis occurs at runtime.

### Internal Concurrency

A lifecycle level may execute its member components concurrently, because
generation determined that no lifecycle ordering edge exists between them.

The exact implementation is an architecture concern. The architectural
requirement is that a level's concurrency is an implementation detail of that
level — never something a plan exposes or a caller schedules or configures.

## Runtime Visibility

The framework shall not expose runtime graph introspection APIs.

Examples of rejected APIs:

```go
lifecycle.Graph()
lifecycle.Plan()
lifecycle.Describe()
lifecycle.ExecutionGroups()
```

Generated source code is the authoritative representation of lifecycle behavior.

Applications inspect generated code rather than query runtime lifecycle metadata.

## Consequences

### Positive

* No runtime lifecycle engine.
* No runtime execution plans.
* No runtime graph traversal.
* Strong alignment with Wire philosophy.
* Improved debuggability.
* Improved IDE navigation.
* Reduced runtime complexity.
* Deterministic execution behavior.

### Negative

* Large dependency graphs may generate large artifacts.
* Lifecycle structure is visible in generated code.
* Changes to orchestration require regeneration.
* Ordering is read from the generated constructor's sequence of level
  declarations, and the walk over those levels is shared code rather than
  generated code. The debuggability above is satisfied by orchestration being
  readable and steppable in ordinary tooling without special lifecycle tools; it
  does not extend to every structural element of the ordering owning a generated
  stack frame of its own.

### Accepted Trade-Off

The project prioritizes:

* Readability.
* Determinism.
* Simplicity.
* Debuggability.

over runtime flexibility.

## Rejected Alternatives

### Runtime Execution Plans

Example:

```go
type Plan struct {
    Startup []Group
    Shutdown []Group
}
```

Rejected because it requires a runtime orchestration engine.

### Runtime Lifecycle Graphs

Rejected because graph analysis already occurs during generation.

Maintaining a second runtime representation provides little value.

### Runtime Topological Sorting

Rejected because lifecycle ordering can be computed during generation.

Repeating the work at runtime introduces unnecessary complexity.

### Generic Lifecycle Engine

Example:

```go
engine.Execute(plan)
```

Rejected because it obscures orchestration behavior behind framework internals.

Generated code should express lifecycle behavior directly.

### Runtime Introspection APIs

Rejected because they require maintaining runtime lifecycle metadata.

Generated code itself is the source of truth.

## Non-Goals

This decision does not provide:

* Runtime lifecycle graph inspection.
* Runtime lifecycle modification.
* Runtime execution planning.
* Dynamic orchestration.
* Runtime dependency analysis.
* Generic workflow execution.

The framework exists to generate lifecycle orchestration code, not to host a lifecycle orchestration engine.
