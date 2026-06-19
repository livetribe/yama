# ADR-009: Adapt Wire Generation Internals

## Status

Accepted

## Context

Yama derives lifecycle orchestration from dependency information expressed in Google Wire provider declarations.

Earlier ADRs establish that Wire is the authoritative source of dependency graph information, but they do not define how Yama obtains that information during generation.

There are two materially different implementation strategies:

* Consume an output produced by Wire, such as generated `wire_gen.go` code or a graph artifact.
* Adapt Wire's generation-time machinery so Yama reads the same source inputs and participates in the same kind of analysis pipeline as Wire.

The project does not intend to inspect generated Wire output or depend on a runtime graph artifact.

Yama needs access to dependency information while generating code, before any runtime artifact exists.

## Decision

Yama shall adapt Wire's code generation internals rather than consume Wire's generated outputs.

Concretely, Yama's generator shall read the same source-level inputs that Wire reads:

* Provider functions.
* Provider sets.
* Bindings.
* Values.
* Struct providers.
* Field providers.
* Injector declarations.
* `wire.Build` calls.

Yama may reuse, fork, internalize, or closely mirror Wire's generation-time machinery, including:

* Package loading through `golang.org/x/tools/go/packages`.
* Wire-compatible source analysis.
* Provider graph construction.
* Solver behavior.
* Code emission patterns.

The dependency graph used by Yama is not an artifact emitted by the Wire command.

It is the generation-time provider graph that Wire's own generator would construct from the application's source inputs.

## Rationale

### No Intermediate Artifact

Parsing generated `wire_gen.go` output would make Yama depend on code shape rather than dependency semantics.

Generated source is intended for compilation and inspection, not as a stable machine-readable graph interface.

By adapting Wire's generation pipeline, Yama avoids inventing or reverse-engineering an intermediate graph format.

### Consistent Graph Semantics

Wire already defines how provider declarations, bindings, values, struct providers, field providers, and injector stubs compose into dependency construction.

Yama should observe the same graph semantics rather than reconstructing them from generated initialization code.

### Generation-Time Availability

Lifecycle ordering must be computed during code generation.

Adapting Wire's analysis path gives Yama access to dependency relationships before runtime and without requiring applications to pass graph data to Yama.

### Output Format Independence

Wire's generated Go code is not a public graph API.

Depending on generated output formatting would make Yama fragile across Wire implementation changes that do not alter Wire semantics.

Yama should couple to Wire's generation-time model instead of to `wire_gen.go` text.

## Consequences

### Positive

* Avoids parsing generated Wire output.
* Avoids runtime graph artifacts.
* Preserves a single source-level dependency declaration model.
* Keeps lifecycle generation aligned with Wire dependency semantics.
* Allows Yama to generate dependency construction and lifecycle orchestration from one generation pass.
* Avoids relying on `wire_gen.go` formatting stability.

### Negative

* Couples Yama to Wire's code generation internals.
* Increases fragility if Wire internal packages or solver behavior change.
* Requires Yama maintainers to understand Wire's analyzer, solver, and emitter architecture.
* May require vendoring, forking, or adapting code that was not designed as a public extension API.
* Makes alternate dependency graph sources harder to support.

### Accepted Trade-Off

The project accepts stronger coupling to Wire internals in exchange for a semantically accurate, generation-time view of the dependency graph.

This coupling is preferable to treating generated Wire output as an implicit API.

## Rejected Alternatives

### Parse Generated `wire_gen.go`

Rejected because generated initialization code is not a stable graph representation.

This approach would be sensitive to formatting, helper extraction, naming, and code emission changes that do not change dependency semantics.

### Consume a Wire Graph Artifact

Rejected because Wire does not provide a stable public graph artifact for downstream generators.

Introducing such an artifact would either require changes outside Yama's control or create a private convention with unclear compatibility guarantees.

### Runtime Graph Introspection

Rejected because Yama is a compile-time generator.

Runtime graph construction or inspection would violate the project's compile-time orchestration model.

### Independent Wire-Like Parser

Rejected because independently reimplementing Wire semantics would create drift risk.

Yama may adapt Wire's internals, but it should not define a separate dependency language that only resembles Wire.

## Architectural Implications

References to the "Wire graph" in other ADRs mean the generation-time provider graph derived from Wire source inputs.

They do not mean:

* A graph object emitted by the Wire command.
* Generated `wire_gen.go` source code.
* A runtime dependency graph.
* A public Wire API for graph inspection.

Yama's generator operates as a peer to Wire's generator, using Wire-compatible analysis machinery to produce Yama-specific generated code.
