# Yama Architecture

## 1. Overview

Yama is a compile-time lifecycle orchestration system for Go applications that use Google Wire. Google Wire remains responsible for dependency construction. Yama runs Google Wire's generator and reads the generated injector (`wire_gen.go`), whose statement order is a valid topological order, then generates ordinary Go code that orchestrates lifecycle operations for the components in that graph.

Yama supports exactly three lifecycle capabilities. Each interface's signature matches the error semantics of the phase it represents:

* `Start(context.Context) error`
* `Quiesce(context.Context)`
* `Stop(context.Context)`

Applications interact with generated lifecycle orchestration through the `Lifecycle` interface, which is composed of the `Starter` and `Stopper` capabilities above — a lifecycle starts and stops the whole graph, so it is the same kind of thing as the participants inside it:

* `Lifecycle.Start(context.Context) error`
* `Lifecycle.Stop(context.Context)`

`Quiesce` is not exposed on `Lifecycle`. `Stop` runs the quiesce pass internally as its unconditional first action, then performs teardown. This holds for both normal shutdown and startup-failure cleanup.

The generated output is executable Go code, not a runtime plan. Startup ordering, shutdown ordering, lifecycle participation, concurrency levels, and interceptor chain construction are all determined during generation and represented directly in generated source. Deadlines are not generated; they come from the context the caller passes to `Start` and `Stop`.

## 2. Design Principles

Yama is implemented around the same operating model as Google Wire:

* Perform graph analysis at generation time.
* Emit explicit Go code.
* Avoid reflection.
* Avoid runtime registration.
* Avoid runtime graph construction.
* Preserve strong typing.
* Keep generated code readable and debuggable.

The runtime lifecycle implementation is intentionally small. It executes generated methods, applies generated timeout policy, calls generated interceptor chains, tracks minimal lifecycle state, and returns lifecycle-level outcomes.

The architecture prefers:

* Compile-time analysis over runtime discovery.
* Generated executable code over interpreted plans.
* Operation-specific interfaces over string or enum dispatch.
* Concrete generated fields over maps and registries.
* Context propagation over framework-specific diagnostic APIs.
* Interceptors over framework-owned logging, metrics, tracing, optionality, or policy systems.

## 3. System Architecture

Yama has two major parts:

1. A generation-time pipeline.
2. A small runtime public API plus generated application-specific implementation.

At generation time, Yama runs `wire gen`, walks the AST of the resulting `wire_gen.go`, derives lifecycle participation and ordering from the generated injector, and emits `lifecycle_gen.go`.

At runtime, the application constructs its dependencies through Google Wire's generated injector and obtains the generated Yama `Lifecycle` value from the generated constructor, which returns `(*App, Lifecycle, error)`. On failure it returns `nil, nil, err`; on success it captures and discards Google Wire's raw cleanup function so teardown runs only through `Lifecycle.Stop`. The `Lifecycle` value owns references to lifecycle-capable component instances, generated interceptor chains, and minimal execution state.

The runtime lifecycle implementation does not discover components. It does not sort dependencies. It does not construct graphs. It does not interpret plans. It executes generated functions whose structure already encodes the generation-time analysis.

## 4. Generation Pipeline

The generation pipeline is:

1. Run `wire gen` to produce `wire_gen.go`. Google Wire resolves binding, interface bindings, cycle detection, and construction ordering.
2. Parse `wire_gen.go` and walk the AST of each injector function.
3. Treat every new variable declaration in the injector body as a creation event (call expressions, `wire.Value`/`InterfaceValue` assignments, and `wire.Struct`/`FieldsOf` struct literals). Top-to-bottom order is a valid topological order.
4. Treat the final root-struct literal returned by the injector (for example `App`) as the manifest of top-level roots, not itself a node. Treat each injector function as an independent graph; never merge them.
5. Derive dependency edges from the arguments each creation event consumes.
6. Detect lifecycle capabilities for each node. A cleanup function is shorthand for a `Stopper`: wrap every Google Wire cleanup function in a synthetic `cleanupAdapter` value implementing `Stopper`, positioned at the cleaned-up value's place for ordering only. A value that both returns a cleanup function and implements `Stopper` becomes two `Stopper` nodes at the same DAG position.
7. Compute lifecycle execution structure:
   * startup levels,
   * quiesce levels,
   * shutdown levels,
   * per-operation interceptor eligibility.
8. Emit the lifecycle file, carrying a provenance header.
9. Format generated Go code with standard Go formatting.

Generation runs one command and produces two files (`wire_gen.go` and the lifecycle file). A CI check regenerates both and diffs them to catch drift.

Generation failures are build-time failures. Examples include Google Wire generation errors, unsupported injector shapes, generated naming conflicts that cannot be resolved within the Yama namespace, or invalid lifecycle analysis results.

## 5. Google Wire Integration

Google Wire provider declarations are the sole source inputs for dependency graph information. Yama does not fork Google Wire, does not import or adapt Google Wire's unexported `internal/` graph-construction packages (Go's `internal/` visibility makes that impossible for an external consumer), and does not duplicate Google Wire's marker types. It references `github.com/google/wire`'s public types directly, so existing Google Wire codebases work unmodified.

Yama obtains ordering by running Google Wire's generator and analyzing the generated injector. By the time `wire gen` emits an injector, Google Wire has resolved provider binding, interface bindings, values, struct and field providers, and cycle detection. The statement order of the generated injector body is a valid topological order, so Yama reuses Google Wire's resolved result rather than reconstructing it.

Yama walks the injector AST rather than a private graph object. Any new variable declaration is a creation event; `wire.Value` and `InterfaceValue` emit assignments, and `wire.Struct` and `FieldsOf` emit struct literals. The final root-struct literal returned by the injector is the manifest of top-level roots and is not itself a node. Multiple injector functions are independent graphs and are never merged.

Google Wire remains responsible for dependency construction semantics. Yama remains responsible for lifecycle orchestration semantics. Coupling to the shape of Google Wire's generated output is guarded by a CI check that regenerates and diffs both files, not by depending on Google Wire's internal APIs.

## 6. Dependency Graph Extraction

The dependency graph Yama uses is the one expressed by the generated injector body. Each graph node represents a value created by a statement in the injector. Each edge represents a value that a later creation event consumes as an argument.

All nodes participate in dependency analysis, including nodes with no lifecycle capability. Dependency-only nodes influence ordering between lifecycle-capable nodes.

Yama extracts:

* the value created by each injector statement,
* the values each creation event consumes,
* the resulting dependency edges,
* the injector's root-struct literal (its output roots),
* concrete values that become lifecycle participants,
* names usable for generated code.

Lifecycle execution includes only nodes whose resulting value implements at least one lifecycle capability. A component with no `Start`, `Quiesce`, or `Stop` method never receives lifecycle callbacks, but it may still impose ordering between lifecycle participants that depend through it.

Yama does not expose the extracted graph publicly. The graph exists only inside the generator.

## 7. Lifecycle Analysis

Lifecycle analysis occurs entirely during generation.

For startup, Yama computes dependency-directed levels over lifecycle-capable participants using the full Google Wire graph. Dependency-only nodes are traversed when deriving ordering. If lifecycle participant A depends on dependency-only node B, and B depends on lifecycle participant C, A is ordered after C even though B does not receive lifecycle callbacks. Participants in the same level have no lifecycle ordering dependency between them and may start concurrently.

For shutdown, Yama computes the reverse dependency-directed levels. Dependents stop before the dependencies they rely on. Independent participants in the same shutdown level may stop concurrently.

For quiesce, Yama computes the same reverse dependency-directed levels over `Quiescer` participants. Quiesce is ordered along dependency edges — dependents quiesce before the dependencies they rely on — because a dependency must not quiesce while a dependent might still call into it. Independent branches quiesce concurrently. Non-`Quiescer` nodes are skipped, but ordering still holds transitively through them.

The analysis also records which generated chain wrappers are needed for each operation. Yama generates no lifecycle configuration and no timeout fields; any deadline is carried by the caller's context.

The result of lifecycle analysis is not emitted as a public or runtime data structure. It is emitted as generated methods, generated structs, generated fields, and generated function calls.

## 8. Generated Code Architecture

Lifecycle code is split between two homes (see ADR-010). Only the **graph-specific**
parts are generated into the application's `lifecycle_gen.go`: the private level
structs that name concrete participants, their Start/Quiesce/Stop ordering methods,
the `YamaInterceptors` input, and the `NewLifecycle` constructor. The **generic
execution plumbing** — interceptor chain construction, the per-node wrapper, the
fail-fast level executor, the boundary runner, `cleanupAdapter`, and the built-in
overrun interceptor — is identical in every application and lives in a Yama-owned
**runtime-support package** that the generated code imports. Keeping ordering in the
generated level structs is what satisfies ADR-004: execution order stays visible in
the application's own source, while the mechanical plumbing is reused rather than
re-emitted per file.

Conceptually, the generated implementation contains:

* references to lifecycle-capable component instances,
* prebuilt start, quiesce, and stop interceptor chains (built by the runtime-support package),
* one generated started flag per lifecycle participant,
* minimal terminal state for lifecycle completion.

The per-participant started flags are the runtime state used to scope quiesce and stop during normal shutdown and startup-failure cleanup. They are concrete generated fields, not entries in a runtime graph or plan.

Generated code contains explicit methods for:

* constructing the lifecycle implementation,
* constructing private generated level values,
* starting each generated level that implements `Starter`,
* quiescing each generated level that implements `Quiescer`,
* stopping each generated level that implements `Stopper`,
* performing startup-failure cleanup.

Chain construction, per-node wrapping, and the built-in per-node overrun interceptor are provided by the runtime-support package and invoked from the generated construction and level methods.

The generated implementation returns only public lifecycle errors. Component-level errors remain inside generated control flow and interceptor observation paths.

Generated code favors readable repetition over compact indirection. Large graphs may generate large files, but the structure should remain navigable by type, method, and field names in ordinary Go IDEs.

## 9. Deadlines and Configuration

Yama generates no lifecycle configuration and owns no lifecycle policy of its own.

There is no generated configuration structure, no start or shutdown deadline field, and no per-participant timeout field. The only deadline is the one carried by the caller's context passed to `Start` and `Stop`. The framework threads that context through the traversal and never lengthens it.

A component that wants a per-node timeout wraps its own `Start`, `Quiesce`, or `Stop` — this is ordinary Go, not a Yama mechanism. Slow-operation and overrun diagnostics are interceptor concerns. Removing lifecycle configuration keeps the framework out of the timeout-policy and configuration business entirely.

## 10. Interceptor Architecture

Interceptors are the primary runtime extension mechanism. They are runtime objects supplied explicitly when the lifecycle implementation is constructed.

Yama uses operation-specific interceptor interfaces:

* start interceptors participate only in `Start`,
* quiesce interceptors participate only in `Quiesce`,
* stop interceptors participate only in `Stop`.

The interceptor interfaces are intentionally not uniform; Yama rejects a single shared interceptor shape. Each interceptor's signature matches the error semantics of the phase it wraps. A start interceptor receives the operation context and a `Starter` next value and returns an `error`, because `Start` can fail. A quiesce interceptor receives the operation context and a `Quiescer` next value and returns nothing, and a stop interceptor receives the operation context and a `Stopper` next value and returns nothing, because those phases have nothing actionable to report. A uniform contract would force `Quiesce` and `Stop` to carry an unused error return or force `Start` to discard the error it must report. Interceptors still observe, suppress, replace, or modify execution by wrapping the `next` value while preserving strong typing.

Generated lifecycle construction accepts global interceptor values and generated per-component interceptor fields through the generated `YamaInterceptors` input. Per-component attachment is strongly typed through generated constructor input fields named for the generated lifecycle participant. A caller attaches an interceptor to a specific component by placing it in that component's generated interceptor field, not by using component names, string keys, runtime lookup, or a registration API. During construction, generated code filters those explicit values by operation-specific interceptor interface and builds the relevant chains.

Generated code builds separate chains for start, quiesce, and stop. Each operation chain combines:

1. global interceptors that implement the operation-specific interceptor interface,
2. per-component interceptors for the participant being invoked,
3. the final component lifecycle method.

Execution order follows registration order. The lifecycle manager does not reorder, prioritize, or rediscover interceptors.

Chain construction happens once during lifecycle manager initialization. Lifecycle execution calls the prebuilt generated chain closures or private chain values. Generated chain code remains strongly typed and operation-specific. It does not use operation enums, string dispatch, reflection, or plugin discovery.

Interceptors may observe execution, modify context, suppress execution, invoke the next element conditionally, alter returned outcomes, and provide diagnostics. Optional participation, environment policy, logging, metrics, tracing, and telemetry are interceptor responsibilities rather than lifecycle manager responsibilities.

Because interceptors require every participant to be invoked through a chain, the wrapper layer is universal: every node is wrapped whether or not an application interceptor is attached to it. Wrapping is not opt-in. The observational overrun logging relies on this same universal wrapping: a built-in, Yama-authored interceptor is attached to every node's chain, and it is what detects and logs a caller's context-deadline overrun — so universal wrapping is what gives that mechanism per-node attribution. This built-in interceptor is an internal implementation detail; applications neither write nor register it, and it adds no public API.

## 11. Error Handling Architecture

Yama exposes a single lifecycle-level error:

* `ErrStartFailed`

Startup returns `ErrStartFailed` if startup cannot complete successfully. `Stop` returns nothing. There is no recovery from shutdown, so there is nothing to return, and there is no `ErrStopFailed`.

Component errors are not returned to the lifecycle caller. Component identity, duration, deadline-overrun detail, and original component errors belong in interceptors and observability integrations.

Startup-failure cleanup does not change the public startup error. If startup fails, generated code runs the same internal shutdown sequence — the quiesce pass, then teardown — for the successfully started components and then returns `ErrStartFailed`. Shutdown produces no error, so cleanup outcomes are observable through interceptors but never change the returned error.

Shutdown runs the quiesce pass and the teardown pass in dependency order, to completion, returning nothing. Because the framework waits for each participant rather than returning early, a hung participant stalls everything after it in the traversal until the orchestrator sends SIGKILL. This is an accepted consequence of never violating reverse-topological ordering.

## 12. Context Propagation Architecture

The caller's context enters `Start` and `Stop`. Generated code derives operation contexts from it when applying timeouts and lifecycle component metadata.

Before invoking interceptors, generated code attaches the current lifecycle participant identity to context. This supports diagnostics, logging, metrics, tracing, and telemetry. The access mechanism is part of the framework-defined interceptor contract: interceptor implementations receive the operation context after component metadata has been attached and can read it with `yama.ComponentFromContext(ctx)`. The lifecycle operation does not need separate context metadata because the operation-specific interceptor method identifies whether the call is Start, Quiesce, or Stop. The context carrier uses unexported keys so components cannot accidentally collide with framework metadata. The accessor exposes component metadata only; it does not expose graph APIs, generated implementation types, lifecycle plans, or component error details.

Interceptors may replace or wrap the context before invoking the next element in the chain. Component lifecycle methods receive the context produced by timeout policy, component context injection, and interceptor processing.

Generated code never extends an existing caller deadline, and generates no deadline of its own. The caller's context deadline, if any, is the only deadline, and it remains authoritative.

Nodes receive an ordinary context that may carry a deadline and handle it with standard Go idioms — including detaching with `context.WithoutCancel` or a fresh context for work that must complete regardless of the deadline. The framework introduces no special contract here beyond "you get a context."

On startup failure, generated code cancels the startup context to prevent additional startup work from being scheduled and to signal in-flight operations in the active level.

## 13. Generated Naming Strategy

Generated implementation artifacts use a Yama-owned namespace and remain private whenever possible.

Private generated names use a consistent prefix such as `yama` followed by descriptive role names. Examples of generated implementation roles include lifecycle implementation, generated level structs such as `yamaLevel001`, component operation wrappers, operation policy structs, chain builders, and `cleanupAdapter` values that wrap Google Wire cleanup functions as `Stopper`.

The implementation plan defines the expected generated names and their responsibilities. Architecture only requires that those names remain in a Yama-owned namespace and that implementation details stay private whenever possible.

The generated participant name is also the component name exposed through `yama.ComponentFromContext(ctx)`. The generator derives it deterministically from the application-facing value or field name when available, otherwise from the provider result type, with deterministic suffixes for collisions.

Generated names must be:

* deterministic across equivalent inputs,
* stable enough for review,
* unique within the generated package,
* readable in stack traces and debugger views,
* clearly distinguishable from application-owned names.

Application-facing generated constructor names may be exported only when the application must reference them directly. Generated implementation details remain unexported. Public framework API names remain limited to the ADR-defined lifecycle interfaces, capability interfaces, interceptor interfaces, and lifecycle-level errors.

## 14. Generated Artifact Layout

Generation produces two files in the target application package: Google Wire's `wire_gen.go` and Yama's `lifecycle_gen.go`. One `go:generate` directive and one command produce both. Google Wire's generator is pinned as a Go tool dependency in `go.mod` and invoked as `go tool wire generate`, so generation is reproducible and needs no assumption about `wire` being on `PATH`. A CI check regenerates both and diffs them against the committed copies to catch drift between `wire_gen.go` and the lifecycle file.

The lifecycle file contains:

* a generated-file header recording provenance (`// Code generated by Yama. DO NOT EDIT.`),
* the build constraint excluding it from the two injection builds,
* package declaration,
* imports required by generated lifecycle code, including the Yama runtime-support package,
* private generated lifecycle implementation type,
* private generated level structs and their ordering methods,
* generated lifecycle constructor integration,
* generated startup, quiesce, and shutdown methods.

The generic execution plumbing (chain construction, per-node wrapping, level execution, boundary running, `cleanupAdapter`, the built-in overrun interceptor) is not emitted here; it lives in the runtime-support package the lifecycle file imports (ADR-010).

Generated artifacts are implementation details. Applications may inspect them for debugging and review, but they should not depend on private generated type names or helper method names.

## 15. Startup Architecture

`Start(ctx)` executes generated level values in dependency order.

Each generated level is a private generated value that implements `Starter` when any of its members participate in startup. The top-level lifecycle treats the level like a component by calling its `Start` method. Inside the level, generated code starts all member participants concurrently. Each participant invocation passes through:

1. participant-specific start timeout policy,
2. generated lifecycle component context injection,
3. the prebuilt start interceptor chain,
4. the component's `Start` method.

A `Start` is expected to return once the component's start side effects have begun; construction already produced an inert, valid value. A component whose start would otherwise block — for example `http.Server.ListenAndServe` or `grpc.Server.Serve` — is responsible for launching the blocking call in its own goroutine and returning, routing the error wherever it needs to go. This is ordinary Go; the framework provides no helper for it.

If all participants in a level succeed, generated code marks those participants as successfully started and proceeds to the next level.

Startup is fail-fast. If any participant in the active level fails, generated code cancels the startup context, stops scheduling further startup levels, waits for in-flight operations in the active level to settle, records which participants started successfully, runs the normal generated shutdown sequence (quiesce pass, then teardown) for those participants, and returns `ErrStartFailed`.

Participants in later levels are never started after a startup failure.

## 16. Quiesce Architecture

Quiesce is invoked internally by generated shutdown processing, as the first pass of `Stop`. Applications do not call quiesce directly, and it is not exposed on `Lifecycle`.

Generated quiesce code invokes `Quiesce` on generated levels that implement `Quiescer`, in reverse dependency order — the same direction as teardown — because a dependency must not quiesce while a dependent might still call into it. Inside each level, generated code invokes independent quiesce-capable members concurrently. Each participant invocation passes through:

1. the caller's context (shared across the whole shutdown),
2. generated lifecycle component context injection,
3. the prebuilt quiesce interceptor chain,
4. the component's `Quiesce` method.

`Quiesce` returns no error. It stops accepting new work and blocks until the component's in-flight work completes. The caller's context deadline is observational: when it fires, the built-in overrun interceptor logs that the participant exceeded its window but the framework keeps waiting for `Quiesce` to return, so a slow participant stalls the participants that depend on it until it returns or SIGKILL arrives.

Idempotent shutdown is the framework's own guarantee: `Stop` runs the quiesce and teardown passes once, so repeated or overlapping `Stop` calls do not re-trigger them. A component whose own "stop accepting new work" step must fire exactly once uses ordinary `sync.Once`; the framework provides no helper for it.

During startup-failure cleanup, generated quiesce code is scoped to participants that successfully started before startup failed.

## 17. Shutdown Architecture

`Stop(ctx)` executes generated shutdown processing:

1. Run the quiesce pass over successfully started quiesce-capable participants in reverse dependency order.
2. Run the teardown pass over successfully started stop-capable participants in reverse dependency order.
3. Return nothing.

Both passes run under the caller's context — the single context passed to `Stop`, whose deadline, if any, spans the whole sequence. The quiesce pass completes before any teardown begins.

Stop levels are generated as private `yamaLevelNNN` values in reverse dependency order over stop-capable participants. A level implements `Stopper` when any of its members participate in shutdown. The top-level lifecycle treats the level like a component by calling its `Stop` method. A dependent stops before the dependency it relies on. Independent participants in the same shutdown level stop concurrently inside the level. Google Wire cleanup functions participate here as `cleanupAdapter` values implementing `Stopper`, so every teardown node is a `Stopper` on a single dispatch path.

Each stop invocation passes through:

1. the caller's context (shared across the whole shutdown),
2. generated lifecycle component context injection,
3. the prebuilt stop interceptor chain,
4. the component's `Stop` method.

`Stop` returns no error. The traversal runs to completion in dependency order; the framework waits for each participant rather than returning early, so a hung participant stalls everything after it until SIGKILL.

## 18. Boundary Nodes

Yama supports two boundary registration points that sit outside the construction
DAG: a **begin** boundary and an **end** boundary. Boundary nodes are peers of the
graph nodes. A boundary node participates in the lifecycle through whichever of
`Starter`, `Quiescer`, and `Stopper` it implements, exactly as a graph node does.
Registering a node in a boundary does not change what it does; it controls only its
execution order relative to the graph.

The lifecycle runs three passes: the Start pass in dependency order, and the
Quiesce and Stop passes in reverse dependency order. In each pass, boundary nodes
bracket the graph — a begin node runs before every graph node in that pass, an end
node runs after every graph node in that pass. A boundary node joins a pass only
when it implements that pass's interface:

```text
Start pass     begin Starters   → graph Starters  → end Starters
Quiesce pass   begin Quiescers  → graph Quiescers → end Quiescers
Stop pass      begin Stoppers   → graph Stoppers  → end Stoppers
```

A boundary registration expresses one thing: this node runs before every graph
node, or after every graph node. It exists so that "run first" or "run last" can be
stated directly instead of by wiring dependency edges against every graph root and
revising them whenever the set of roots changes.

Each boundary is a flat, unordered set. Nodes in the same set have no ordering
relationship and may execute concurrently; Yama makes no ordering guarantee among
them. A boundary node has no dependency relationship to any graph node. Anything
that needs an ordering relative to specific nodes has a real dependency
relationship and belongs in the construction graph, not in a boundary set.

Boundary execution is best-effort. A boundary node that returns an error or panics
does not prevent the pass from proceeding — a failed begin node still lets the
graph run, and a failed end node does not change the outcome. This matches the
shutdown model, in which shutdown returns nothing and always runs to completion.
Like graph nodes, boundary nodes are wrapped, so a boundary failure or overrun is
observable through interceptors.

In each pass, boundary nodes run under the same caller context as the graph nodes
and share its deadline. Yama gives a boundary node no budget of its own and does
not preempt it; a slow begin node consumes budget that the rest of the pass would
otherwise have. This is a documented consequence, not a mitigated one.

Because boundary nodes are part of the passes rather than a separate step, they
bracket those passes wherever they run. This includes the internal startup-failure
cleanup, which reuses the Quiesce and Stop passes over successfully started
participants; there is no separate boundary execution path.

Boundary nodes are supplied as runtime objects when the lifecycle value is
constructed, using the `WithBeginNode` and `WithEndNode` options alongside
interceptors. They are not derived from the Wire graph, are not lifecycle graph
participants, and never appear in generated startup, quiesce, or teardown levels.

Two cases illustrate the boundaries without defining them:

* An in-process readiness flip is a begin node. As a `Quiescer`, its `Quiesce`
  runs before every graph node quiesces, so the readiness probe flips to failing
  before the graph drains and the routing layer stops sending new work. (See
  Appendix A for how this relates to the `preStop` hook, which is the primary
  mechanism and is out of Yama's scope.)
* A metrics flush that must run after everything else is an end node — a `Stopper`
  whose `Stop` runs after every graph node stops. This holds only if it owns its
  transport. If its exporter depends on a Wire-constructed connection or pool, the
  flush has a genuine dependency on a graph node and belongs in the construction
  graph as an ordinary teardown participant, not in the end boundary.

## 19. Concurrency Model

Concurrency opportunities are computed during generation.

Generated startup and shutdown code is divided into explicit private level values. Participants in the same level may execute concurrently because lifecycle analysis has determined that no lifecycle ordering edge exists between them for that operation. Levels execute sequentially and each level is invoked through the lifecycle capability interfaces it implements.

Generated quiesce code follows the same reverse dependency ordering as teardown; it does not ignore dependency ordering. Independent branches quiesce concurrently, but a participant quiesces only after the dependents that rely on it.

Generated code may use standard Go synchronization primitives such as goroutines, wait groups, channels, mutexes, or `errgroup`-style helpers. These helpers are private implementation details. They coordinate only the generated operations for the current lifecycle call and do not represent runtime graph state or runtime execution plans.

Startup uses fail-fast coordination within a level. The quiesce and teardown passes coordinate ordered work that waits for each participant to return, so ordering is never violated to reclaim liveness.

## 20. Timeout Handling

Yama generates no deadline and owns no timeout policy. The only deadline is the one carried by the caller's context passed to `Start` and `Stop`. That single context is threaded through the whole traversal — for shutdown, the quiesce pass and teardown pass share it. Generated code never lengthens the caller's deadline.

The deadline is observational. When it fires, the built-in overrun interceptor, attached to every node, logs that a participant exceeded its window — giving per-node attribution — but the framework does not return early. It continues waiting for the participant's operation to actually complete. Returning early would let the traversal reach a participant's dependencies while that participant might still be using them, violating reverse-topological ordering. Preserving ordering is chosen over liveness; external liveness is bounded by the orchestrator's SIGKILL.

There is no timeout error and no framework-owned remediation. Because one context spans the whole shutdown, it is accepted that a slow quiesce can consume the window and leave teardown little time. Overruns are observable through interceptors, which receive the operation context and observe the operation.

A component that needs a per-node timeout wraps its own `Start`, `Quiesce`, or `Stop` — this is ordinary Go, not a Yama mechanism. Nodes handle the deadline with ordinary Go idioms; work that must complete regardless can detach with `context.WithoutCancel` or a fresh context.

This fixes the boundary of Yama's responsibility. Yama guarantees phase ordering and deadline propagation: it runs each phase in the correct dependency order and threads the caller's context, with its deadline, through every node without lengthening it. Honoring that context is the node's responsibility. Yama does not preempt an uncooperative node; a node that ignores its deadline is stalled only by the orchestrator's SIGKILL. This holds for graph nodes and boundary nodes alike.

## 21. Observability Architecture

Yama is observability-tool agnostic. It does not expose logger, tracer, meter, health, or readiness APIs.

Observability is implemented through interceptors and lifecycle metadata propagated in context. Generated code provides enough metadata for interceptors to associate observations with operation and component execution without exposing public graph APIs.

Interceptors can measure duration, log failures, emit metrics, start traces, record deadline overruns, and record component diagnostics. A built-in, Yama-authored interceptor attached to every node is where a deadline overrun is detected and logged, with per-node attribution; it is an internal detail that adds no public API. The lifecycle manager itself only determines lifecycle outcomes.

## 22. Failure Scenarios

Startup participant failure:

* cancel startup context,
* stop scheduling startup levels,
* wait for in-flight operations in the active level,
* quiesce successfully started participants,
* stop successfully started participants,
* return `ErrStartFailed`.

Caller's start context deadline exceeded:

* handled as startup participant failure,
* exposed publicly as `ErrStartFailed`,
* details available only through interceptors and observability.

Quiesce participant that does not return:

* the framework keeps waiting; it does not return early,
* the caller's context-deadline overrun is logged with per-node attribution,
* dependencies protected by the participant do not quiesce until it returns,
* nothing is returned to the caller.

Stop participant that does not return:

* the framework keeps waiting; the traversal does not proceed past it,
* the overrun is logged,
* nothing is returned to the caller.

Hung participant:

* a participant that never returns stalls everything after it in the traversal until the orchestrator sends SIGKILL,
* this is intentional and follows from preserving reverse-topological ordering.

Cleanup after startup failure:

* run the quiesce pass and teardown pass for successfully started participants,
* return `ErrStartFailed`,
* expose cleanup diagnostics through interceptors.

Caller context cancellation:

* propagate cancellation through generated operation contexts,
* during startup, treat resulting failures as startup failure and return `ErrStartFailed`,
* during shutdown, the passes still run to completion in order and return nothing.

## 23. Performance Considerations

Yama moves expensive lifecycle work to generation time:

* running `wire gen`,
* parsing the generated injector,
* lifecycle capability analysis,
* topological level computation from the injector's statement order,
* shutdown level computation,
* chain shape generation.

Runtime avoids graph traversal, sorting, discovery, registration, reflection, and plan interpretation.

Runtime overhead consists primarily of:

* calling generated methods,
* launching generated concurrent operations,
* applying context timeouts,
* invoking interceptor chains,
* tracking which participants started successfully,
* aggregating lifecycle-level success or failure state internally.

Generated code may be larger for large dependency graphs. This is an accepted trade-off for readability, determinism, and ordinary Go debugging.

## 24. Testing Strategy

Testing covers the generator, generated code shape, and runtime behavior of generated lifecycle implementations.

Generator tests should use real source-file fixtures containing Google Wire injector inputs, run `wire gen`, and assert that the AST walk of the resulting `wire_gen.go` derives the correct ordering. Fixtures should cover provider functions, provider sets, bindings, values, struct providers, field providers, injector declarations, transitive ordering through dependency-only nodes, and cleanup functions wrapped as `cleanupAdapter`. The tests assert that lifecycle analysis produces correct startup levels, quiesce levels, shutdown levels, and chain construction, and that a value which both returns a cleanup function and implements `Stopper` yields two `Stopper` nodes at the same DAG position. These tests operate on the generated injector and generated source, not on a private runtime graph API.

Generated-code tests should compile generated packages and exercise the public lifecycle surface. They should verify startup ordering, shutdown ordering, quiesce-before-teardown behavior, reverse-topological quiesce ordering, concurrent independent branches, startup fail-fast behavior, startup-failure cleanup, that shutdown returns nothing and runs to completion, observational-deadline overrun logging, and that only `ErrStartFailed` is returned. Golden-file tests should cover generated source shape for representative graphs so readability, naming stability, and chain construction remain reviewable.

Interceptor tests should verify separate operation chains, the non-uniform signatures, global and per-component composition, registration-order execution, context modification, behavior suppression, and outcome modification.

Error tests should verify that callers receive only `ErrStartFailed`, that `Stop` returns nothing, and that component errors and deadline overruns are available only to interceptors or test observability hooks.

Drift tests should regenerate `wire_gen.go` and the lifecycle file and diff them against the committed copies, so a change in Google Wire's generated output shape fails visibly at the drift boundary rather than at runtime.

## 25. Rejected Alternatives

Runtime lifecycle plans are rejected because generated executable Go code is the authoritative lifecycle representation.

Runtime graph construction and runtime graph introspection are rejected because the dependency ordering is a generation-time artifact derived from Google Wire's generated injector.

Adapting or forking Google Wire's generation internals is rejected because Google Wire's graph-construction and solver logic live under `internal/` packages that external code cannot import, and depending on them would require a fork that grows fragile as those internals change. Yama runs `wire gen` and walks the generated injector instead, coupling to Google Wire's output shape (guarded by a CI drift check) rather than to its internals.

Lifecycle registration APIs are rejected because they create a second source of truth beside Google Wire.

Reflection-based discovery is rejected because it weakens type safety and hides behavior behind runtime machinery.

Service locators are rejected because they obscure dependency relationships and move dependency validation from compile time to runtime.

Configuration frameworks are rejected because configuration acquisition, parsing, validation, precedence, and reload are outside lifecycle orchestration.

Framework-owned observability, health, and readiness systems are rejected because observability and operational policy belong in interceptors and application systems.

Additional lifecycle phases and lifecycle phase customization are rejected because Yama supports exactly `Start`, `Quiesce`, and `Stop`.

Generic workflow engines, plugin systems, retry frameworks, backoff frameworks, and lifecycle plan interpreters are rejected because Yama is a focused compile-time lifecycle orchestration system.

## 26. Future Work

Future work may add observability integrations or lifecycle visualization tooling if those additions preserve the accepted architecture.

Any future enhancement must keep Google Wire provider declarations as the authoritative dependency source, keep lifecycle analysis at generation time, preserve the generated-code-first philosophy, emit executable Go code rather than runtime plans, avoid reflection and runtime registration, preserve the fixed `Start`/`Quiesce`/`Stop` lifecycle model, and avoid expanding the stable public API beyond necessary lifecycle participation and interception concepts.

## Appendix A. Running Under Kubernetes

Yama runs graceful shutdown when the process receives SIGTERM; it does not encode any orchestrator-specific behavior. The guidance below is documentation, not a mechanism inside the library.

**The readiness-to-routing gap belongs to a `preStop` hook, not to Yama.** When a pod's readiness probe flips to failing, there is a delay before the load balancer actually stops routing to it. That gap is covered by a Kubernetes `preStop` hook, which delays SIGTERM, not by an artificial sleep inside the library. Yama's `Quiesce` intentionally adds no such delay, because doing so would encode an orchestrator-specific assumption into a general-purpose library.

**The shutdown budget must fit inside `terminationGracePeriodSeconds`.** After SIGTERM, Kubernetes sends SIGKILL once the grace period elapses. The deadline on the context the caller passes to `Stop`, plus any `preStop` delay, should fit within `terminationGracePeriodSeconds`, because SIGKILL — not the observational deadline — is what ultimately bounds shutdown.

**An in-process readiness flip is a begin boundary node, not a graph node.** The primary mechanism for the readiness-to-routing gap is the `preStop` hook above, which fires before SIGTERM and is out of Yama's scope; the total shutdown budget, including any `preStop` delay, must fit within `terminationGracePeriodSeconds`. If an application additionally wants to flip readiness from inside the process, that flip should run before the graph quiesces, which makes it a begin boundary node (see Boundary Nodes): a `Quiescer` registered in the begin boundary, rather than a `Quiescer` wired into the construction graph. Modeling it as a graph node would require wiring dependency edges only to force it to the front of the quiesce pass; the begin boundary expresses that position directly.

## Appendix B. Long-Lived Work

Yama cannot manufacture time that is not there. The observational deadline waits for a node, but the orchestrator's SIGKILL can still land mid-operation regardless of how the node handles its context.

Work that must not be lost therefore has to be crash-safe and resumable at the storage layer — write-ahead logging, an outbox, or atomic and replayable writes — so that a process killed mid-operation can recover on restart. This is application-level guidance, not a Yama feature. A node may detach from the shutdown deadline with `context.WithoutCancel` to finish an in-flight unit of work, but it cannot rely on always being allowed to finish, so durability must not depend on shutdown completing.

## Appendix C. Public API Reference

ADR-007 records the decision to keep Yama's public API minimal and the reasoning behind it. This appendix is the current, authoritative enumeration of that surface — `package yama`'s complete set of exported symbols — and is the document to update whenever that surface changes. ADR-007 is not re-edited to keep this list current; its own listings illustrate the decision as accepted, not a live catalog.

### Lifecycle Type

```go
type Lifecycle interface {
    Starter
    Stopper
}
```

`Lifecycle` is an interface composed of the capability interfaces below: it starts
and stops the whole graph, so it is the same kind of thing as the participants
inside it. Its implementation is private and owned by the runtime-support package;
applications receive a `Lifecycle` and never implement or construct one. Because
`Starter` and `Stopper` are themselves public and frozen, the composition adds no
compatibility commitment beyond those already made.

The generated constructor returns the application and its `Lifecycle` together:

```go
app, lifecycle, err := NewLifecycle(interceptors, WithBeginNode(n1), WithEndNode(n2))
```

### Capability Interfaces

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

### Interceptor Interfaces

```go
type StartInterceptor interface {
    Start(ctx context.Context, next Starter) error
}

type QuiesceInterceptor interface {
    Quiesce(ctx context.Context, next Quiescer)
}

type StopInterceptor interface {
    Stop(ctx context.Context, next Stopper)
}
```

### Context Accessor

```go
func ComponentFromContext(ctx context.Context) (Component, bool)

type Component struct {
    Name string
}
```

### Errors

```go
var ErrStartFailed error
```

### Helpers

```go
func WithBeginNode(node any) Option // register a node that runs before the graph in each pass
func WithEndNode(node any) Option   // register a node that runs after the graph in each pass
func RunUntilSignal(lc Lifecycle, signals ...os.Signal) error // Start, wait for a signal, then Stop
```

`WithBeginNode` and `WithEndNode` take `any` because Go has no `Starter | Quiescer | Stopper` union type; Yama detects the node's capabilities by type assertion.

### Explicitly Not Public

Generated artifacts (private level structs, generated constructor internals) and the exported symbols of the Yama-owned runtime-support package (ADR-010) are not part of this surface, even though the latter are technically exported so generated code can reach them. Applications should not depend on anything outside the list above.
