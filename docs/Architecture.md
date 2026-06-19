# Yama Architecture

## 1. Overview

Yama is a compile-time lifecycle orchestration system for Go applications that use Google Wire. Google Wire remains responsible for dependency construction. Yama reads the same Google Wire provider declarations and builds a Google Wire-compatible generation-time provider graph, then generates ordinary Go code that orchestrates lifecycle operations for components in that graph.

Yama supports exactly three lifecycle capabilities:

* `Start(context.Context) error`
* `Drain(context.Context) error`
* `Stop(context.Context) error`

Applications interact with generated lifecycle orchestration through the public lifecycle surface:

* `Lifecycle.Start(context.Context) error`
* `Lifecycle.Stop(context.Context) error`

`Drain` is an internal phase. The generated lifecycle implementation invokes it before `Stop` during normal shutdown and startup-failure cleanup.

The generated output is executable Go code, not a runtime plan. Startup ordering, shutdown ordering, lifecycle participation, timeout application, concurrency levels, and interceptor chain construction are all determined during generation and represented directly in generated source.

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

At generation time, Yama loads the application's Google Wire injector inputs, builds the Google Wire-compatible provider graph, derives lifecycle participation and ordering, and emits `lifecycle_gen.go`.

At runtime, the application constructs its dependencies through generated injector code that follows Google Wire construction semantics and also wires the generated Yama lifecycle value. This may be emitted by Yama's generator in place of separate Google Wire output for the same injector, but the generated construction code remains dependency construction code and the generated lifecycle methods remain lifecycle orchestration code. The lifecycle value owns references to lifecycle-capable component instances, generated configuration, generated interceptor chains, and minimal execution state.

The runtime lifecycle implementation does not discover components. It does not sort dependencies. It does not construct graphs. It does not interpret plans. It executes generated functions whose structure already encodes the generation-time analysis.

## 4. Generation Pipeline

The generation pipeline is:

1. Load packages and injector declarations using Google Wire-compatible source loading.
2. Analyze provider functions, provider sets, bindings, values, struct providers, field providers, injector declarations, and `wire.Build` calls.
3. Build the same generation-time provider graph semantics Google Wire uses for dependency construction.
4. Solve the injector graph using Google Wire-compatible solver behavior.
5. Extract dependency edges from the solved provider graph.
6. Detect lifecycle capabilities for each graph node.
7. Compute lifecycle execution structure:
   * startup levels,
   * drain participants,
   * shutdown levels,
   * per-operation timeout fields,
   * per-operation interceptor eligibility.
8. Emit Go code containing dependency construction integration and lifecycle orchestration.
9. Format generated Go code with standard Go formatting.

Generation failures are build-time failures. Examples include unresolved providers, Google Wire graph errors, unsupported graph shapes, generated naming conflicts that cannot be resolved within the Yama namespace, or invalid lifecycle analysis results.

## 5. Google Wire Integration

Google Wire provider declarations are the sole source inputs for dependency graph information. Yama does not parse `wire_gen.go`, consume a graph artifact emitted by Google Wire, or ask the application for a second lifecycle graph.

Yama obtains graph information by adapting Google Wire's generation-time internals. The generator reads the same source-level inputs Google Wire reads:

* provider functions,
* provider sets,
* interface bindings,
* values,
* struct providers,
* field providers,
* injector declarations,
* `wire.Build` calls.

Yama internalizes a Google Wire-compatible generation package for package loading, provider graph construction, solver behavior, and the code-emission patterns needed by Yama output. This internal package may originate from Google Wire source, but once adopted it is maintained as Yama generator infrastructure rather than selected dynamically at runtime or treated as an interchangeable backend. Google Wire compatibility is protected by generator tests against Google Wire source semantics, not by parsing Google Wire's generated output.

Google Wire remains responsible for dependency construction semantics. Yama remains responsible for lifecycle orchestration semantics. The generated artifact may be produced in the same generation pass as dependency construction output, but construction and lifecycle orchestration remain separate responsibilities in the generated code.

## 6. Dependency Graph Extraction

The dependency graph Yama uses is the generation-time provider graph derived from Google Wire source inputs. Each graph node represents a provided value in the solved injector graph. Each edge represents a provider dependency required to construct that value.

All graph nodes participate in dependency analysis, including nodes with no lifecycle capability. Dependency-only nodes influence ordering between lifecycle-capable nodes.

Yama extracts:

* provider result identity,
* provider input dependencies,
* solved dependency edges,
* injector output roots,
* concrete values that become lifecycle participants,
* source-level names usable for generated code.

Lifecycle execution includes only nodes whose resulting value implements at least one lifecycle capability. A component with no `Start`, `Drain`, or `Stop` method never receives lifecycle callbacks, but it may still impose ordering between lifecycle participants that depend through it.

Yama does not expose the extracted graph publicly. The graph exists only inside the generator.

## 7. Lifecycle Analysis

Lifecycle analysis occurs entirely during generation.

For startup, Yama computes dependency-directed levels over lifecycle-capable participants using the full Google Wire graph. Dependency-only nodes are traversed when deriving ordering. If lifecycle participant A depends on dependency-only node B, and B depends on lifecycle participant C, A is ordered after C even though B does not receive lifecycle callbacks. Participants in the same level have no lifecycle ordering dependency between them and may start concurrently.

For shutdown, Yama computes the reverse dependency-directed levels. Dependents stop before the dependencies they rely on. Independent participants in the same shutdown level may stop concurrently.

For drain, Yama records all `Drainer` participants. Drain ignores dependency ordering and executes all drain participants concurrently before any stop operation begins.

The analysis also records which participants need operation-specific generated timeout fields and which generated chain wrappers are needed for each operation.

The result of lifecycle analysis is not emitted as a public or runtime data structure. It is emitted as generated methods, generated structs, generated fields, and generated function calls.

## 8. Generated Code Architecture

Generated lifecycle code is organized around a private Yama-owned implementation type. Conceptually, the generated implementation contains:

* references to lifecycle-capable component instances,
* generated lifecycle configuration,
* generated operation policies,
* prebuilt start, drain, and stop interceptor chains,
* one generated started flag per lifecycle participant,
* minimal terminal state for lifecycle completion.

The per-participant started flags are the runtime state used to scope drain and stop during normal shutdown and startup-failure cleanup. They are concrete generated fields, not entries in a runtime graph or plan.

Generated code contains explicit methods for:

* constructing the lifecycle implementation,
* constructing operation-specific interceptor chains,
* constructing private generated level values,
* starting each generated level that implements `Starter`,
* draining each generated level that implements `Drainer`,
* stopping each generated level that implements `Stopper`,
* applying operation-specific timeouts,
* performing startup-failure cleanup.

The generated implementation returns only public lifecycle errors. Component-level errors remain inside generated control flow and interceptor observation paths.

Generated code favors readable repetition over compact indirection. Large graphs may generate large files, but the structure should remain navigable by type, method, and field names in ordinary Go IDEs.

## 9. Lifecycle Policy Architecture

Lifecycle policy is represented by generated, strongly typed configuration structures and generated private policy helpers.

Generated configuration structures are application-specific artifacts. They contain lifecycle-related fields only. A participant that implements `Starter` receives a start timeout field. A participant that implements `Drainer` receives a drain timeout field. A participant that implements `Stopper` receives a stop timeout field. Components with no lifecycle capability receive no lifecycle configuration.

Generated private policy helpers convert those fields into operation behavior. The primary policy behavior is timeout application. Timeout policy is operation-specific and participant-specific. It is encoded with concrete generated fields rather than maps, string keys, or dynamic lookup.

Configuration construction remains the application's responsibility. Yama consumes generated configuration values but does not load, parse, validate, discover, bind, reload, or prioritize configuration.

## 10. Interceptor Architecture

Interceptors are the primary runtime extension mechanism. They are runtime objects supplied explicitly when the lifecycle implementation is constructed.

Yama uses operation-specific interceptor interfaces:

* start interceptors participate only in `Start`,
* drain interceptors participate only in `Drain`,
* stop interceptors participate only in `Stop`.

Each operation-specific interceptor returns `error`, matching the lifecycle operation it wraps. A start interceptor receives the operation context and a `Starter` next value and returns the start outcome. A drain interceptor receives the operation context and a `Drainer` next value and returns the drain outcome. A stop interceptor receives the operation context and a `Stopper` next value and returns the stop outcome. This return value is what allows interceptors to observe, suppress, replace, or modify operation outcomes while preserving strong typing.

Generated lifecycle construction accepts global interceptor values and generated per-component interceptor fields through the generated `YamaInterceptors` input. Per-component attachment is strongly typed through generated constructor input fields named for the generated lifecycle participant. A caller attaches an interceptor to a specific component by placing it in that component's generated interceptor field, not by using component names, string keys, runtime lookup, or a registration API. During construction, generated code filters those explicit values by operation-specific interceptor interface and builds the relevant chains.

Generated code builds separate chains for start, drain, and stop. Each operation chain combines:

1. global interceptors that implement the operation-specific interceptor interface,
2. per-component interceptors for the participant being invoked,
3. the final component lifecycle method.

Execution order follows registration order. The lifecycle manager does not reorder, prioritize, or rediscover interceptors.

Chain construction happens once during lifecycle manager initialization. Lifecycle execution calls the prebuilt generated chain closures or private chain values. Generated chain code remains strongly typed and operation-specific. It does not use operation enums, string dispatch, reflection, or plugin discovery.

Interceptors may observe execution, modify context, suppress execution, invoke the next element conditionally, alter returned outcomes, and provide diagnostics. Optional participation, environment policy, logging, metrics, tracing, and telemetry are interceptor responsibilities rather than lifecycle manager responsibilities.

## 11. Error Handling Architecture

Yama exposes lifecycle-level errors only:

* `ErrStartFailed`
* `ErrStopFailed`

Startup returns `ErrStartFailed` if startup cannot complete successfully. Shutdown returns `ErrStopFailed` if drain or stop encounters one or more failures.

Component errors are not returned to the lifecycle caller. Component identity, operation identity, duration, timeout detail, and original component errors belong in interceptors and observability integrations.

Startup-failure cleanup does not change the public startup error. If startup fails, generated code performs best-effort drain and stop for successfully started components and then returns `ErrStartFailed`. Cleanup failures are observable through interceptors but are not propagated as component errors or joined errors.

Drain or stop failures during startup-failure cleanup do not change the returned error from `ErrStartFailed`.

Shutdown is best effort. Generated code continues remaining drain and stop work after failures and returns `ErrStopFailed` after shutdown processing completes if any shutdown failure occurred.

## 12. Context Propagation Architecture

The caller's context enters `Start` and `Stop`. Generated code derives operation contexts from it when applying timeouts and lifecycle component metadata.

Before invoking interceptors, generated code attaches the current lifecycle participant identity to context. This supports diagnostics, logging, metrics, tracing, and telemetry. The access mechanism is part of the framework-defined interceptor contract: interceptor implementations receive the operation context after component metadata has been attached and can read it with `yama.ComponentFromContext(ctx)`. The lifecycle operation does not need separate context metadata because the operation-specific interceptor method identifies whether the call is Start, Drain, or Stop. The context carrier uses unexported keys so components cannot accidentally collide with framework metadata. The accessor exposes component metadata only; it does not expose graph APIs, generated implementation types, lifecycle plans, or component error details.

Interceptors may replace or wrap the context before invoking the next element in the chain. Component lifecycle methods receive the context produced by timeout policy, component context injection, and interceptor processing.

Generated code never extends an existing caller deadline. When a generated lifecycle timeout is configured, the effective deadline is the earlier of the caller's deadline and the lifecycle timeout. If the caller's context is already more restrictive, it remains authoritative.

On startup failure, generated code cancels the startup context to prevent additional startup work from being scheduled and to signal in-flight operations in the active level.

## 13. Generated Naming Strategy

Generated implementation artifacts use a Yama-owned namespace and remain private whenever possible.

Private generated names use a consistent prefix such as `yama` followed by descriptive role names. Examples of generated implementation roles include lifecycle implementation, generated level structs such as `yamaLevel001`, component operation wrappers, operation policy structs, and chain builders.

The implementation plan defines the expected generated names and their responsibilities. Architecture only requires that those names remain in a Yama-owned namespace and that implementation details stay private whenever possible.

The generated participant name is also the component name exposed through `yama.ComponentFromContext(ctx)`. The generator derives it deterministically from the application-facing value or field name when available, otherwise from the provider result type, with deterministic suffixes for collisions.

Generated names must be:

* deterministic across equivalent inputs,
* stable enough for review,
* unique within the generated package,
* readable in stack traces and debugger views,
* clearly distinguishable from application-owned names.

Application-facing generated constructor and configuration names may be exported only when the application must reference them directly. Generated implementation details remain unexported. Public framework API names remain limited to the ADR-defined lifecycle interfaces, capability interfaces, interceptor interfaces, and lifecycle-level errors.

## 14. Generated Artifact Layout

The primary generated artifact is `lifecycle_gen.go` in the target application package selected by the injector generation context.

The file contains:

* standard generated-file header,
* package declaration,
* imports required by generated lifecycle code,
* generated lifecycle configuration structures,
* private generated lifecycle implementation type,
* private generated policy structures or helpers,
* private generated component operation wrappers,
* private generated interceptor chain construction,
* generated lifecycle constructor integration,
* generated startup, drain, and shutdown methods,
* generated concurrency helper functions where needed.

Generated artifacts are implementation details. Applications may inspect them for debugging and review, but they should not depend on private generated type names or helper method names.

## 15. Startup Architecture

`Start(ctx)` executes generated level values in dependency order.

Each generated level is a private generated value that implements `Starter` when any of its members participate in startup. The top-level lifecycle treats the level like a component by calling its `Start` method. Inside the level, generated code starts all member participants concurrently. Each participant invocation passes through:

1. participant-specific start timeout policy,
2. generated lifecycle component context injection,
3. the prebuilt start interceptor chain,
4. the component's `Start` method.

If all participants in a level succeed, generated code marks those participants as successfully started and proceeds to the next level.

Startup is fail-fast. If any participant in the active level fails, generated code cancels the startup context, stops scheduling further startup levels, waits for in-flight operations in the active level to quiesce, records which participants started successfully, runs the normal generated shutdown sequence for those participants, and returns `ErrStartFailed`.

Participants in later levels are never started after a startup failure.

## 16. Drain Architecture

Drain is invoked internally by generated shutdown processing. Applications do not call drain directly.

Generated drain code invokes `Drain` on generated levels that implement `Drainer`. Drain does not use dependency ordering between individual components. Inside each draining level, generated code invokes all successfully started drain-capable members concurrently. Each participant invocation passes through:

1. participant-specific drain timeout policy,
2. generated lifecycle component context injection,
3. the prebuilt drain interceptor chain,
4. the component's `Drain` method.

Drain is best effort. Failures and timeouts are recorded as shutdown failures but do not stop other drain work and do not prevent stop execution.

During startup-failure cleanup, generated drain code is scoped to participants that successfully started before startup failed.

## 17. Shutdown Architecture

`Stop(ctx)` executes generated shutdown processing:

1. Drain successfully started drain-capable participants.
2. Stop successfully started stop-capable participants in reverse dependency order.
3. Return `ErrStopFailed` if any drain or stop operation failed.

Stop levels are generated as private `yamaLevelNNN` values in reverse dependency order over stop-capable participants. A level implements `Stopper` when any of its members participate in shutdown. The top-level lifecycle treats the level like a component by calling its `Stop` method. A dependent stops before the dependency it relies on. Independent participants in the same shutdown level stop concurrently inside the level.

Each stop invocation passes through:

1. participant-specific stop timeout policy,
2. generated lifecycle component context injection,
3. the prebuilt stop interceptor chain,
4. the component's `Stop` method.

Shutdown is best effort. Failures in one participant do not prevent remaining participants from stopping.

## 18. Concurrency Model

Concurrency opportunities are computed during generation.

Generated startup and shutdown code is divided into explicit private level values. Participants in the same level may execute concurrently because lifecycle analysis has determined that no lifecycle ordering edge exists between them for that operation. Levels execute sequentially and each level is invoked through the lifecycle capability interfaces it implements.

Generated drain code executes all eligible drain participants concurrently because drain intentionally ignores dependency ordering.

Generated code may use standard Go synchronization primitives such as goroutines, wait groups, channels, mutexes, or `errgroup`-style helpers. These helpers are private implementation details. They coordinate only the generated operations for the current lifecycle call and do not represent runtime graph state or runtime execution plans.

Startup uses fail-fast coordination within a level. Shutdown and drain use best-effort coordination that waits for all scheduled work to finish.

## 19. Timeout Handling

Timeouts are generated as strongly typed configuration fields and applied per participant operation.

For each operation invocation, generated code computes an effective context:

* if no operation timeout is configured, use the incoming context,
* if the incoming context has an earlier deadline, keep the incoming deadline,
* if the configured operation timeout is earlier, derive a context with that timeout.

Timeout expiration is treated as an ordinary lifecycle failure. It does not produce public timeout-specific errors and does not trigger framework-owned remediation.

Timeout diagnostics are observable through interceptors because interceptors receive the operation context and observe the returned operation outcome.

## 20. Observability Architecture

Yama is observability-tool agnostic. It does not expose logger, tracer, meter, health, or readiness APIs.

Observability is implemented through interceptors and lifecycle metadata propagated in context. Generated code provides enough metadata for interceptors to associate observations with operation and component execution without exposing public graph APIs.

Interceptors can measure duration, log failures, emit metrics, start traces, capture timeout behavior, and record component diagnostics. The lifecycle manager itself only determines lifecycle outcomes.

## 21. Failure Scenarios

Startup participant failure:

* cancel startup context,
* stop scheduling startup levels,
* wait for in-flight operations in the active level,
* drain successfully started participants,
* stop successfully started participants,
* return `ErrStartFailed`.

Startup timeout:

* handled as startup participant failure,
* exposed publicly as `ErrStartFailed`,
* details available only through interceptors and observability.

Drain failure:

* continue remaining drain operations,
* continue to stop,
* include the failure in shutdown outcome,
* return `ErrStopFailed` from `Stop` if shutdown was application-initiated.

Stop failure:

* continue remaining stop operations,
* return `ErrStopFailed` after shutdown completes.

Cleanup failure after startup failure:

* continue best-effort cleanup,
* return `ErrStartFailed`,
* expose cleanup diagnostics through interceptors.

Caller context cancellation:

* propagate cancellation through generated operation contexts,
* treat resulting operation failures according to the active lifecycle operation,
* return `ErrStartFailed` for startup failure or `ErrStopFailed` for shutdown failure.

## 22. Performance Considerations

Yama moves expensive lifecycle work to generation time:

* package loading,
* provider graph construction,
* graph solving,
* lifecycle capability analysis,
* topological level computation,
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

## 23. Testing Strategy

Testing covers the generator, generated code shape, and runtime behavior of generated lifecycle implementations.

Generator tests should use real source-file fixtures containing Google Wire injector inputs. Fixtures should cover provider functions, provider sets, bindings, values, struct providers, field providers, injector declarations, and transitive ordering through dependency-only nodes. The tests assert that lifecycle analysis produces correct startup levels, drain participants, shutdown levels, timeout fields, and chain construction. These tests operate on generation-time data structures and generated source, not public runtime graph APIs.

Generated-code tests should compile generated packages and exercise the public lifecycle surface. They should verify startup ordering, shutdown ordering, drain-before-stop behavior, concurrent independent branches, startup fail-fast behavior, startup-failure cleanup, best-effort shutdown, timeout handling, and lifecycle-level errors. Golden-file tests should cover generated source shape for representative graphs so readability, naming stability, and chain construction remain reviewable.

Interceptor tests should verify separate operation chains, global and per-component composition, registration-order execution, context modification, behavior suppression, and outcome modification.

Error tests should verify that callers receive only `ErrStartFailed` and `ErrStopFailed`, while component errors are available only to interceptors or test observability hooks.

Compatibility tests should protect Google Wire integration by comparing Yama's internalized Google Wire-compatible analysis against Google Wire source semantics for supported inputs. When Google Wire changes upstream behavior, these tests should fail at generation or golden-output boundaries, making the maintenance impact visible before runtime.

## 24. Rejected Alternatives

Runtime lifecycle plans are rejected because generated executable Go code is the authoritative lifecycle representation.

Runtime graph construction and runtime graph introspection are rejected because the dependency graph is a generation-time artifact derived from Google Wire source inputs.

Parsing `wire_gen.go` is rejected because generated Google Wire output is not a stable graph interface. Yama adapts Google Wire's generation-time machinery instead.

Lifecycle registration APIs are rejected because they create a second source of truth beside Google Wire.

Reflection-based discovery is rejected because it weakens type safety and hides behavior behind runtime machinery.

Service locators are rejected because they obscure dependency relationships and move dependency validation from compile time to runtime.

Configuration frameworks are rejected because configuration acquisition, parsing, validation, precedence, and reload are outside lifecycle orchestration.

Framework-owned observability, health, and readiness systems are rejected because observability and operational policy belong in interceptors and application systems.

Additional lifecycle phases and lifecycle phase customization are rejected because Yama supports exactly `Start`, `Drain`, and `Stop`.

Generic workflow engines, plugin systems, retry frameworks, backoff frameworks, and lifecycle plan interpreters are rejected because Yama is a focused compile-time lifecycle orchestration system.

## 25. Future Work

Future work may add lifecycle configuration options, configuration binding helpers, observability integrations, or lifecycle visualization tooling if those additions preserve the accepted architecture.

Any future enhancement must keep Google Wire provider declarations as the authoritative dependency source, keep lifecycle graph analysis at generation time, preserve the generated-code-first philosophy, emit executable Go code rather than runtime plans, avoid reflection and runtime registration, preserve the fixed `Start`/`Drain`/`Stop` lifecycle model, and avoid expanding the stable public API beyond necessary lifecycle participation and interception concepts.
