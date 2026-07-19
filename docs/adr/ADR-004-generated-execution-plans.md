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
* Runtime execution metadata.
* Runtime lifecycle state.
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
* Runtime execution metadata.
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

Generated lifecycle levels may execute their member components concurrently.

Example:

```go
func (lvl *yamaLvl001) Start(ctx context.Context) error {
    var g errgroup.Group

    g.Go(func() error {
        return lvl.rateLimiter.Start(ctx)
    })

    g.Go(func() error {
        return lvl.auditLog.Start(ctx)
    })

    return g.Wait()
}
```

The exact implementation is an architecture concern.

The architectural requirement is that concurrency remains encapsulated inside generated lifecycle components rather than exposed through runtime lifecycle plans.

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
