# ADR-002: Google Wire as the Authoritative Dependency Graph

## Status

Accepted

## Context

The lifecycle framework requires a dependency graph to derive:

* Startup ordering.
* Shutdown ordering.
* Parallel execution groups.
* Dependency relationships.

A central architectural question is where this dependency graph originates.

Several possible approaches exist:

* A custom lifecycle graph definition language.
* Lifecycle-specific registration APIs.
* Struct tags or annotations.
* Reflection-based graph discovery.
* Runtime registration.
* Existing compile-time dependency injection graphs.

The project already assumes that applications use Google Wire for dependency injection.

Google Wire constructs a compile-time dependency graph from source-level provider declarations and generates initialization code (`wire_gen.go`) from that graph. By the time Google Wire emits that code, it has already performed binding resolution, interface binding, and cycle detection.

Introducing a second graph definition mechanism would require application developers to maintain dependency information in multiple locations.

A second mechanism would recreate the duplication that the project intends to eliminate.

## Decision

Google Wire provider declarations shall be the sole authoritative source inputs for dependency graph information.

The lifecycle framework shall derive lifecycle orchestration from the Google Wire injector generated from those declarations.

The framework shall not introduce:

* A lifecycle graph DSL.
* A lifecycle registration API.
* A lifecycle-specific dependency model.
* Runtime dependency graph construction.
* Alternate dependency graph providers.

Lifecycle orchestration shall be generated from the Google Wire injector produced for dependency construction. Dependency construction and lifecycle orchestration therefore derive from one set of provider declarations.

## Rationale

### Single Source of Truth

Applications already describe dependencies in Google Wire.

Creating a second graph for lifecycle purposes would duplicate information. The two graphs could then drift apart.

The dependency graph should exist exactly once.

### Reduced Cognitive Load

Developers should not need to learn or maintain two separate graphs:

* A dependency injection graph.
* A separate lifecycle graph.

Instead, the lifecycle framework should derive lifecycle behavior from the dependency graph that the application already has.

### Reduced Surface Area

Supporting multiple graph sources would require:

* Additional APIs.
* Additional validation.
* Additional documentation.
* Additional testing.

Restricting the framework to Google Wire keeps the public API small and focused.

### Deterministic Behavior

Google Wire already performs compile-time graph validation.

The lifecycle framework derives lifecycle orchestration from the graph that Google Wire validates. Lifecycle behavior is therefore deterministic on the same terms.

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
* Makes future support for alternate graph sources more difficult.

### Accepted Trade-Off

The project prioritizes simplicity, determinism, and maintainability over broad ecosystem compatibility.

Supporting multiple graph sources would increase flexibility. It would also introduce substantial complexity that the project's goals do not justify.

## Rejected Alternatives

### Custom Lifecycle Graph DSL

Example:

```go
var LifecycleGraph = lifecycle.Graph(
    lifecycle.Component[*Router](),
    lifecycle.Component[*KafkaConsumer](),
)
```

The project rejects a lifecycle graph DSL because it duplicates information that Google Wire already holds.

### Runtime Registration

Example:

```go
func init() {
    lifecycle.Register(router)
}
```

The project rejects runtime registration because it creates a second source of truth. Runtime registration also moves validation from compile time to runtime.

### Reflection-Based Discovery

Example:

```go
lifecycle.Scan(container)
```

The project rejects reflection-based discovery because it introduces reflection, implicit behavior, and runtime graph construction.

### Struct Tags or Annotations

Example:

```go
type Router struct {
    _ struct{} `lifecycle:"depends-on=kafka"`
}
```

The project rejects struct tags and annotations because Google Wire already holds the dependency information. An application should not repeat it.

### Multiple Graph Providers

Example:

```go
type GraphProvider interface {
    Graph() *Graph
}
```

The project rejects a graph provider interface because it introduces abstraction without a demonstrated need.

The project deliberately optimizes for a single graph source.

## Architectural Implications

The lifecycle framework may assume:

* A complete dependency graph exists.
* Dependency relationships are known at generation time.
* Google Wire's generator validates dependency cycles during `wire gen`.
* Dependency types are known at generation time.

As a result, the lifecycle framework does not need to implement:

* Graph discovery.
* Dependency registration.
* Runtime graph validation.
* Graph mutation.

Its responsibility is to derive lifecycle ordering from Google Wire's provider declarations rather than from an independent graph source.

## Non-Goals

This decision does not imply:

* Support for other dependency injection frameworks.
* A framework-agnostic lifecycle API.
* Runtime dependency graph construction.
* Migration tooling from non-Google Wire systems.

The project explicitly assumes that Google Wire provider declarations are the authoritative dependency source for the application.
