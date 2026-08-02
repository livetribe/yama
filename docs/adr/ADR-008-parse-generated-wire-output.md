# ADR-008: Derive Lifecycle Ordering by Parsing Wire's Generated Injector

## Status

Accepted

## Context

Yama derives lifecycle orchestration from dependency information expressed in Google Wire provider declarations.

Earlier ADRs establish that Google Wire is the authoritative source of dependency graph information. They do not define how Yama obtains that information during generation.

There are two materially different implementation strategies:

* Adapt Google Wire's generation-time machinery so Yama reads the same source inputs and reimplements the same graph-construction and solving pipeline as Google Wire.
* Run Google Wire's generator and derive lifecycle ordering from the generated injector it produces.

Adapting Google Wire's internal machinery is not practically available to an external consumer. Google Wire's graph-construction, solver, and emission logic are declared in `internal/` packages, which Go's visibility rules make inaccessible to code outside the Google Wire module. To use them, Yama would have to fork or vendor Google Wire. Yama would then maintain a divergent copy of code that was never designed as a public extension API.

At the same time, Google Wire already resolves everything Yama needs. By the time `wire gen` emits an injector, Google Wire has resolved provider binding, interface bindings, values, and struct and field providers. It has also detected any cycle. The statement order of the injector it emits is a valid topological order of the dependency graph.

## Decision

Yama shall run `wire gen` and derive lifecycle ordering by walking the AST of the resulting `wire_gen.go`.

Yama shall reference `github.com/google/wire`'s public types directly so existing Google Wire codebases work unmodified.

Yama shall not:

* Fork Google Wire.
* Import or adapt Google Wire's unexported `internal/` graph-construction packages.
* Duplicate Google Wire's marker types.
* Reimplement Google Wire's solver.

### How the generated injector is read

The statement order of the generated injector body is a valid topological order. Walking it top to bottom visits dependencies before dependents.

Any new variable declaration in the injector body is treated as a creation event, not only call expressions. `wire.Value` and `InterfaceValue` emit assignments. `wire.Struct` emits a struct literal. `FieldsOf` emits a selector expression that reads the field from its parent value. All of these introduce a provided value and are lifecycle-graph components.

Each injector function is an independent graph. Multiple injector functions are never merged into a single graph.

### Cleanup functions

Yama supports Google Wire's cleanup functions (`func()` returns) for backward compatibility. Support lets an existing Google Wire codebase's teardown keep working without being rewritten as `Stopper` implementations. A cleanup is not a lifecycle capability of its own.

A cleanup function is folded into the teardown of the value it cleans up, at that value's DAG position. That teardown runs the cleanup first, then the value's own `Stop` if it implements `Stopper`. It does not become a separate component. Yama does not modify the injected type graph.

The two run in sequence rather than concurrently, because they touch the same resource. Yama has no principled basis for the order: a cleanup that releases what `Stop` still needs and a cleanup that releases what `Stop` waits on are both ordinary. It therefore fixes one order and stays out of the way. A provider whose value implements `Stopper` *and* returns a cleanup owns the interaction between them.

Cleanup functions do not pass through interceptor chains. A cleanup is a plain `func()`. It takes no context, so it cannot observe a deadline. It carries no component identity. An interceptor would therefore have nothing to attribute an observation to. Interception applies to components that participate through the lifecycle interfaces.

### Re-emitting the injector body

The generated lifecycle constructor **re-emits** the injector's construction body
rather than calling the injector. This is forced, not stylistic. An injector's
signature returns only the graph's roots, and lifecycle orchestration needs every
value that occupies a level — including values that are injector-locals and reach
no root's signature. Calling the injector would surrender them. Re-emitting keeps
every value in scope, and the constructor swaps only the tail: instead of
returning Google Wire's aggregated cleanup, each provider's cleanup is placed at
its own value's position in the ordering.

Re-emission is also what makes `wire_gen.go` disposable. Because the constructor
reproduces the construction, nothing at runtime calls Wire's injector, so the
injector need not survive generation.

Re-emission is faithful on the error path as well as the success path. When a
provider returns an error, Google Wire's injector calls the cleanups of the
values already built before returning, so that a failed construction leaks
nothing. The re-emitted body reproduces that unwinding as Wire emitted it. This
matters because the constructor's own tail is the only part that changes: the
lifecycle takes ownership of teardown for a construction that *succeeded*, and a
construction that failed never produces a `Lifecycle` for anything to take
ownership through.

Yama reproduces a re-emitted `wire.Value` or `wire.InterfaceValue` as its own
value expression. It does not emit a reference to the package-level
`_wire…Value` variable that Wire declared. That variable is declared in
`wire_gen.go`, and Yama removes that file. Yama therefore reads the resolved
initializer out of `wire_gen.go` during parsing, and emits the initializer in
place of the reference.

Component locals keep the names Wire gave them. Cleanup locals do not: Wire names
them positionally — `cleanup`, `cleanup2`, `cleanup3` — which is adequate inside
an injector that immediately aggregates them all into one closure, and misleading
in a constructor that keeps each one in scope and hands it to a different level.
Each cleanup is therefore rebound to a name derived from the value it releases.
That derivation is the only identifier Yama invents in the generated file, and it
carries the same obligations as any generated identifier: deterministic across
equivalent inputs, stable enough to review across regenerations, and unique within
the generated package.

### Generation and drift

Generation is one `go:generate` directive that invokes Yama, which derives a Google Wire injector from each lifecycle stub (ADR-011), runs Google Wire over it, parses the generated injector, and emits the lifecycle file. The lifecycle file is the sole committed output and carries a provenance header:

```go
// Code generated by Yama. DO NOT EDIT.
```

The `wire_gen.go` Yama generates is a transient intermediate, not a committed artifact. Yama needs the injector only to derive ordering. The lifecycle file re-emits that construction. Nothing at runtime calls Wire's injector. An application constructs and runs through Yama's generated lifecycle constructor.

Adopting Yama is additive. An application adds a lifecycle stub file. It also adds a `//go:generate` directive that invokes Yama. That directive belongs in a committed file, because the transient `wire_gen.go` cannot carry one. Adoption modifies no file the application already has.

An application may also want Wire injectors of its own. It then keeps its `wire.go`, its own `//go:generate` directive naming Wire's command, and its committed `wire_gen.go`. Wire never sees the stub file, because a build tag that Wire does not set guards that file. Yama's run leaves a committed `wire_gen.go` as it found it.

An application may instead give Yama sole ownership of generation. It then points its existing directive at Yama and stops committing `wire_gen.go`. That is a choice, and not a requirement.

Google Wire writes `wire_gen.go` into the package directory, and it offers no way to redirect the file. Yama therefore removes `wire_gen.go` after it emits the lifecycle file. The derived-injector file is transient on the same terms. Google Wire generates from a package, so Yama writes the derived injector into the package directory. Yama removes that file with `wire_gen.go`.

Removal is non-destructive for both. Yama removes only a file it wrote. A file already present under either name is moved aside before generation and restored afterward, so generation never overwrites or deletes a file Yama does not own. Both removals run even when a later step fails, so a failed run leaves the package directory as it found it.

A CI check regenerates the lifecycle file and diffs it against the committed copy to catch drift. A change in Yama's parser, its analysis, or its emitter changes the lifecycle file's content, so it surfaces at that diff.

## Rationale

### No Fork, No Internal Coupling

Google Wire's graph-construction internals live under `internal/` and are unavailable to external code without forking. Parsing the generated injector avoids coupling to unexported packages that were never a public API, and lets Yama depend on `github.com/google/wire`'s public types instead.

### Google Wire Already Resolved the Graph

By the time `wire gen` emits an injector, Google Wire has performed binding resolution, interface binding, and cycle detection. The generated injector's statement order is a valid topological order. Yama reuses that result rather than reconstructing it.

### Unmodified Existing Codebases

Because Yama references Google Wire's public types and runs its generator, existing Google Wire codebases work without modification. Cleanup functions keep working as written.

## Consequences

### Positive

* Avoids forking or vendoring Google Wire.
* Avoids coupling to Google Wire's unexported `internal/` packages.
* Works with existing Google Wire codebases unmodified. Adoption adds a stub file
  and a `go:generate` directive, and changes no file the application already has.
* Reuses Google Wire's own binding, interface binding, and cycle detection.

### Negative

* Depends on the shape of Google Wire's generated injector output. Google Wire is archived, so that shape is fixed rather than a moving target.
* Requires running `wire gen` as a generation step.
* Requires a CI drift check to detect divergence in the committed lifecycle file.

### Accepted Trade-Off

The project accepts a dependency on the structure of Google Wire's generated injector, in exchange for avoiding a fork and reusing Google Wire's already-resolved dependency graph. Google Wire is archived, so the structure Yama parses cannot change under it. This coupling is preferable to the alternative, which is to import `internal/` packages that Go's visibility rules make inaccessible.

## Rejected Alternatives

### Adapt or Fork Google Wire Internals

Rejected because Google Wire's graph-construction, solver, and emission logic are declared in `internal/` packages that external code cannot import. To use them, Yama would have to fork or vendor Google Wire and maintain a divergent copy. That copy becomes more fragile whenever Google Wire's internal packages or solver behavior change.

### Duplicate Google Wire's Marker Types

Rejected because Yama can reference `github.com/google/wire`'s public types directly. Duplicating the marker types would fragment the ecosystem and require applications to migrate.

### Consume a Google Wire Graph Artifact

Rejected because Google Wire does not provide a stable public graph artifact for downstream generators. The generated injector, produced by the same command applications already run, is the available artifact.

### Independent Wire-Like Parser

Rejected because independently reimplementing Google Wire's binding and solving semantics would create drift risk. Yama runs Google Wire's own generator and reads its output rather than defining a separate dependency language that only resembles Google Wire.

## Architectural Implications

References to the "Wire graph" in other ADRs mean the dependency ordering expressed by the generated injector body in `wire_gen.go`.

They do not mean:

* A graph object emitted by a Google Wire graph API.
* Google Wire's unexported internal provider graph.
* A runtime dependency graph.
* A public Google Wire API for graph inspection.

Yama's generator runs Google Wire's generator as a step and analyzes the resulting generated injector to produce Yama-specific lifecycle code.
