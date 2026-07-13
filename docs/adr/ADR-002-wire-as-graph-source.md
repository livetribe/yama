# ADR-002: Google Wire as the Authoritative Dependency Graph

## Status

Accepted

## Context

The lifecycle framework requires a dependency graph in order to derive:

* Startup ordering.
* Shutdown ordering.
* Parallel execution groups.
* Dependency relationships.
* Critical path analysis.

A central architectural question is where this dependency graph originates.

Several possible approaches exist:

* A custom lifecycle graph definition language.
* Lifecycle-specific registration APIs.
* Struct tags or annotations.
* Reflection-based graph discovery.
* Runtime registration.
* Existing compile-time dependency injection graphs.

The project already assumes applications use Google Wire for dependency injection.

Google Wire constructs a compile-time dependency graph from source-level provider declarations and generates initialization code (`wire_gen.go`) from that graph.

Yama runs Google Wire's generator and derives lifecycle ordering from the generated injector. Google Wire has already resolved provider binding, interface bindings, and cycle detection when it emits the injector, and the statement order of the generated injector body is a valid topological order of the dependency graph.

ADR-008 records the implementation strategy and trade-offs for that generator approach.

Introducing a second graph definition mechanism would require application developers to maintain dependency information in multiple locations.

This would recreate the duplication the project is intended to eliminate.

## Decision

Google Wire provider declarations shall be the sole authoritative source inputs for dependency graph information.

The lifecycle framework shall derive lifecycle orchestration from the Google Wire injector generated from those declarations.

The framework shall not introduce:

* A lifecycle graph DSL.
* A lifecycle registration API.
* A lifecycle-specific dependency model.
* Runtime dependency graph construction.
* Alternate dependency graph providers.

Lifecycle orchestration shall be generated from the Google Wire injector produced for dependency construction, so both derive from one set of provider declarations.

## Clarification

References to Google Wire as the authoritative dependency graph mean that Google Wire provider declarations are the authoritative source inputs for dependency information.

Yama does not introduce a second dependency declaration mechanism. It obtains dependency ordering by running Google Wire's generator over those declarations and analyzing the generated injector.

The dependency ordering Yama uses is the topological order already expressed by the generated injector body. It is not a runtime graph or a public graph API. ADR-008 records how the generated injector is analyzed.

## Rationale

### Single Source of Truth

Applications already describe dependencies in Google Wire.

Creating a second graph for lifecycle purposes would duplicate information and create opportunities for drift.

The dependency graph should exist exactly once.

### Reduced Cognitive Load

Developers should not need to learn or maintain:

* A dependency injection graph.
* A separate lifecycle graph.

Instead, lifecycle behavior should emerge from the dependency graph already present in the application.

### Reduced Surface Area

Supporting multiple graph sources would require:

* Additional APIs.
* Additional validation.
* Additional documentation.
* Additional testing.

Restricting the framework to Google Wire keeps the public API small and focused.

### Deterministic Behavior

Google Wire already performs compile-time graph validation.

By deriving lifecycle plans from the same graph, lifecycle behavior inherits the same deterministic structure.

### Alignment with Project Philosophy

The project is intentionally narrow in scope.

Its purpose is not to become a generic lifecycle engine.

Its purpose is to generate lifecycle orchestration for applications that already use Google Wire.

This constraint removes significant complexity from the design.

## Consequences

### Positive

* Eliminates duplicate graph definitions.
* Keeps lifecycle planning deterministic.
* Simplifies generator implementation.
* Reduces public API surface area.
* Avoids lifecycle-specific graph maintenance.
* Leverages existing Google Wire adoption and tooling.
* Maintains compile-time validation.

### Negative

* Requires applications to use Google Wire.
* Limits adoption by applications using other dependency injection approaches.
* Depends on the shape of Google Wire's generated injector output, guarded by a CI drift check rather than by Google Wire's internal APIs.
* Makes future support for alternate graph sources more difficult.

### Accepted Trade-Off

The project prioritizes simplicity, determinism, and maintainability over broad ecosystem compatibility.

Supporting multiple graph sources would increase flexibility but introduce substantial complexity that is not justified by the project's goals.

## Rejected Alternatives

### Custom Lifecycle Graph DSL

Example:

```go
var LifecycleGraph = lifecycle.Graph(
    lifecycle.Component[*Router](),
    lifecycle.Component[*KafkaConsumer](),
)
```

Rejected because it duplicates information already present in Google Wire.

### Runtime Registration

Example:

```go
func init() {
    lifecycle.Register(router)
}
```

Rejected because it creates a second source of truth and moves validation from compile time to runtime.

### Reflection-Based Discovery

Example:

```go
lifecycle.Scan(container)
```

Rejected because it introduces reflection, implicit behavior, and runtime graph construction.

### Struct Tags or Annotations

Example:

```go
type Router struct {
    _ struct{} `lifecycle:"depends-on=kafka"`
}
```

Rejected because dependency information already exists in Google Wire and should not be repeated.

### Multiple Graph Providers

Example:

```go
type GraphProvider interface {
    Graph() *Graph
}
```

Rejected because it introduces abstraction without a demonstrated need.

The project is intentionally optimized for a single graph source.

## Architectural Implications

The lifecycle framework may assume:

* A complete dependency graph exists.
* Dependency relationships are known at generation time.
* Dependency cycles are validated by Google Wire's generator during `wire gen`.
* Dependency types are known at generation time.

As a result, the lifecycle framework does not need to implement:

* Graph discovery.
* Dependency registration.
* Runtime graph validation.
* Graph mutation.

Its responsibility is to run Google Wire's generator and analyze the resulting generated injector.

## Non-Goals

This decision does not imply:

* Support for other dependency injection frameworks.
* A framework-agnostic lifecycle API.
* Runtime dependency graph construction.
* Migration tooling from non-Google Wire systems.

The project is explicitly designed around the assumption that Google Wire provider declarations are the authoritative dependency source for the application.
