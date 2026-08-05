# Yama Architecture

## 1. Overview

Yama is a compile-time lifecycle orchestration system for Go applications that use Google Wire. Google Wire remains responsible for dependency construction. Yama runs Google Wire's generator and reads the generated injector (`wire_gen.go`). The statement order in that file is a valid topological order. Yama then generates ordinary Go code that orchestrates lifecycle operations for the components in that graph.

Yama supports exactly three lifecycle capabilities. Each interface's signature matches the error semantics of the phase it represents:

* `Start(context.Context) error`
* `Quiesce(context.Context)`
* `Stop(context.Context)`

Applications interact with generated lifecycle orchestration through the `Lifecycle` interface. `Lifecycle` is composed of the `Starter` and `Stopper` capabilities above. A lifecycle starts and stops the whole graph, so it is the same kind of thing as the components inside it:

* `Lifecycle.Start(context.Context) error`
* `Lifecycle.Stop(context.Context)`

`Quiesce` is not exposed on `Lifecycle`. `Stop` runs the quiesce pass internally as its unconditional first action, then performs teardown. This holds for both normal shutdown and startup-failure cleanup.

The generated output is executable Go code, not a runtime plan. Startup ordering, shutdown ordering, lifecycle participation, and concurrency levels are all determined during generation and stated directly in generated source. Deadlines are not generated. They come from the context the caller passes to `Start` and `Stop`.

## 2. Design Principles

Yama is implemented around the same operating model as Google Wire:

* Perform graph analysis at generation time.
* Emit explicit Go code.
* Avoid reflection.
* Avoid runtime registration.
* Avoid runtime graph construction.
* Preserve strong typing.
* Keep generated code readable and debuggable.

The runtime lifecycle implementation is intentionally small. It walks the levels the generated constructor declared, calls the prebuilt interceptor chains, tracks minimal lifecycle state, and returns lifecycle-level outcomes. It computes no ordering of its own.

The architecture prefers:

* Compile-time analysis over runtime discovery.
* Generated executable code over interpreted plans.
* Operation-specific interfaces over string or enum dispatch.
* Concrete typed state over maps and registries.
* Context propagation over framework-specific diagnostic APIs.
* Interceptors over framework-owned logging, metrics, tracing, optionality, or policy systems.

## 3. System Architecture

Yama has two major parts:

1. A generation-time pipeline.
2. A small runtime public API plus generated application-specific implementation.

At generation time, Yama runs `wire gen` and walks the AST of the resulting `wire_gen.go`. It derives lifecycle participation and ordering from the generated injector and emits `lifecycle_gen.go`.

At runtime, the application constructs its dependencies and obtains its `Lifecycle` from one call to the generated constructor. That constructor returns `(*App, Lifecycle, error)`. The constructor re-emits the injector's construction instead of calling it, so Google Wire's injector is not present in the built package (§14). Each provider cleanup is routed to its own value's position in the ordering, so teardown runs only through `Lifecycle.Stop`. On construction failure the constructor returns `nil, nil, err`. The `Lifecycle` value owns the declared levels, the wrapped components in them, the prebuilt interceptor chains, and minimal execution state.

The runtime lifecycle implementation does not discover components. It does not sort dependencies. It does not construct graphs. It does not interpret plans. It executes a level list. The generated source that built the list fixed the list's membership and order.

## 4. Generation Pipeline

The generation pipeline is:

1. Read the package's lifecycle stub file to learn which constructors exist, under what name and signature. Also learn which providers build each one's graph (ADR-011).
2. Derive one Google Wire injector per stub. Write the derived injectors into the package directory as a transient, build-tagged file (ADR-011).
3. Run `wire gen` to produce `wire_gen.go`. Google Wire resolves binding, interface bindings, cycle detection, and construction ordering.
4. Parse `wire_gen.go`. Walk the AST of the injector derived for each stub.
5. Treat every new variable declaration in the injector body as a creation event (call expressions, `wire.Value`/`InterfaceValue` assignments, `wire.Struct` struct literals, and `FieldsOf` selector expressions). Top-to-bottom order is a valid topological order.
6. Treat each injector function as an independent graph. Never merge them.
7. Derive dependency edges from the arguments each creation event consumes.
8. Detect lifecycle capabilities for each component. Yama supports a Google Wire cleanup function for backward compatibility. A cleanup is not a lifecycle capability. Yama folds it into the teardown of the value it cleans up, at that value's DAG position. That teardown runs before the value's own `Stop`.
9. Compute lifecycle execution structure:
   * one dependency-ordered level list,
   * each member's teardown form.
10. Emit the lifecycle file. Include a provenance header.
11. Format generated Go code with standard Go formatting.

Generation runs one command and produces one committed file, the lifecycle file. The derived-injector file and `wire_gen.go` are both transient intermediates. Yama removes both after generation. If either file already existed and Yama did not create it, Yama preserves that file. A CI check regenerates the lifecycle file and diffs it against the committed copy to catch drift.

Generation failures are build-time failures. Examples include:

* Google Wire generation errors,
* a stub whose `wire.Build` call states providers that Google Wire cannot build the stub's result from,
* unsupported injector shapes,
* two injectors in one package whose result types share an unqualified name but denote different types,
* invalid lifecycle analysis results.

## 5. Google Wire Integration

Google Wire provider declarations are the sole source inputs for dependency graph information. Yama does not fork Google Wire. Yama does not import or adapt Google Wire's unexported `internal/` graph-construction packages. Go's `internal/` visibility makes that impossible for an external consumer. Yama does not duplicate Google Wire's marker types. It references `github.com/google/wire`'s public types directly, so existing Google Wire codebases work unmodified.

Yama obtains ordering by running Google Wire's generator and analyzing the generated injector. By the time `wire gen` emits an injector, Google Wire has resolved provider binding, interface bindings, values, struct and field providers, and cycle detection. The statement order of the generated injector body is a valid topological order, so Yama reuses Google Wire's resolved result rather than reconstructing it.

Yama walks the injector AST rather than a private graph object. Any new variable declaration is a creation event. `wire.Value` and `InterfaceValue` emit assignments, `wire.Struct` emits a struct literal, and `FieldsOf` emits a selector expression reading the field from its parent value. Multiple injector functions are independent graphs and are never merged.

Google Wire remains responsible for dependency construction semantics. Yama remains responsible for lifecycle orchestration semantics. Coupling to the shape of Google Wire's generated output is guarded by a CI check that regenerates the lifecycle file and diffs it against the committed copy, not by depending on Google Wire's internal APIs.

## 6. Dependency Graph Extraction

The dependency graph Yama uses is the one expressed by the generated injector body. Each graph component represents a value created by a statement in the injector. Each edge represents a value that a later creation event consumes as an argument.

All components participate in dependency analysis, including components with no lifecycle capability. Dependency-only components influence ordering between lifecycle-capable components.

Yama extracts:

* the value created by each injector statement,
* the values each creation event consumes,
* the resulting dependency edges,
* the injector's return value,
* concrete values that become lifecycle components,
* names usable for generated code.

Lifecycle execution includes every lifecycle-capable component and every dependency-only component with cleanup. Each of these occupies a level. A component with neither trait occupies no level, though it may still impose ordering between lifecycle components that depend through it. Callbacks are limited to the capabilities a component implements. A dependency-only component with cleanup receives no callbacks. Only its cleanup runs.

Yama does not expose the extracted graph publicly. The graph exists only inside the generator.

## 7. Lifecycle Analysis

Lifecycle analysis occurs entirely during generation.

Yama checks the type of each component the injector creates: the component is capable if the type implements a lifecycle interface.

Yama computes one dependency-ordered level list over every component that occupies a level: every lifecycle-capable component, and every dependency-only component with cleanup. A component with neither trait is traversed when deriving ordering but occupies no level of its own. If lifecycle-capable component A depends on such a component B, and B depends on lifecycle-capable component C, A is ordered after C even though B occupies no level. Components in the same level have no ordering dependency between them and may run concurrently.

Startup runs that list forward. Quiesce and shutdown run it back, so dependents quiesce and stop before the dependencies they rely on. A dependency must not quiesce while a dependent might still call into it. A component takes no part in a pass whose capability interface it does not implement, and ordering still holds transitively through the ones it lacks.

The analysis also records, for each component, which of the three teardown forms it takes: the value alone, the value paired with its provider's cleanup, or that cleanup standing alone. Yama generates no lifecycle configuration and no timeout fields. Any deadline is carried by the caller's context.

The result of lifecycle analysis is not emitted as a public or runtime data structure, and not as a data structure at all. It is emitted as a sequence of generated function calls that declare each level and name its members.

## 8. Generated Code Architecture

Lifecycle code is split between two homes (see ADR-010). The **graph-specific** part is generated into the application's `lifecycle_gen.go`. The **generic execution machinery** is identical in every application and lives in a Yama-owned **runtime-support package** that the generated file imports.

The generated file emits **no types and no methods**. Its whole content is one constructor per lifecycle stub (ADR-011), and each constructor does three things:

1. **re-emits its derived injector's construction body**, so every value the graph builds is in scope, including the injector-locals that Wire's own signature does not return (ADR-008);
2. **declares the levels**, in dependency order, naming each level's members;
3. **seals the declaration** and returns the application beside its `Lifecycle`.

Level membership and level order are expressed as a call chain against the
runtime-support package's builder, not as generated types:

```go
return app,
    rt.NewLifecycleBuilder(opts...).
        NextLevel().
        WithComponents(base1).
        WithCleanup(base2Cleanup).
        WithCleanableComponent(base3, base3Cleanup).
        NextLevel().
        WithComponents(mid2).
        WithComponents(root2).
        NextLevel().
        WithComponents(root3).
        Build(),
    nil
```

`NextLevel` starts a new level. Every member added after it belongs to that level.
`NextLevel` and the `With…` methods return the builder, so the calls chain.
`Build` ends the chain and returns the `Lifecycle`. The whole declaration is one
expression in the constructor's `return` statement, and the constructor declares
no local variable for the builder or for the `Lifecycle`.

The order in which levels are added is the dependency order: the first level
added starts first, and both shutdown passes walk the levels back. Within a
level, the order members are added carries no meaning, since they run
concurrently. The three member forms are the only per-member decision generation
makes and the runtime cannot: `WithComponents` for a value whose provider
returned no cleanup, `WithCleanableComponent` for a value paired with the Google
Wire cleanup its provider returned, and `WithCleanup` for that cleanup standing
alone at the position of a value that implements no capability of its own.

`WithInterceptors` (ADR-005), `WithBeginComponents`, and `WithEndComponents`
(ADR-009) are public, non-generated `Option`s passed through to the builder.
Interceptors attach globally rather than per component, and boundary
components are supplied at the call site, so no generated input exists for either.

This is what satisfies ADR-004: level membership and level order are literal,
reviewable statements in the application's own source, while the mechanical
execution is reused rather than re-emitted per file. ADR-004 records why the
ordered level list the runtime then holds is not the runtime plan it rejects.

### What the runtime-support package holds

* the builder that generated code calls to declare levels,
* the representation of a level and the ordered walk over the levels: forward to
  start, backward for the quiesce and teardown passes,
* the intra-level executor, which runs a level's members concurrently and waits for
  all of them,
* interceptor chain construction and the per-component wrapper that attaches
  component identity,
* the per-component started state and the gates that use it,
* boundary placement: the begin set opens as the first level and the end set is
  appended as the last, so generated code declares only the graph's own levels,
* the cleanup wrappers, paired-with-component and standalone,
* the built-in per-component overrun interceptor.

The per-component started state is what scopes quiesce and teardown during both
normal shutdown and startup-failure cleanup, and it lives in the runtime-support
package rather than in generated fields. Every wrapped component carries it, so
shutdown gates uniformly: a `Starter` is gated out of both shutdown passes unless
its `Start` returned without error or panic. A component that implements no
`Starter` has no start to fail and is never gated out. A component in a level the
failed startup never reached is not gated at all. The traversal simply stops at
the failing level and never walks past it, so unreached levels take no part in
either pass.

The generated constructor returns only Google Wire's own construction error.
Lifecycle execution returns only public lifecycle errors. Component-level errors
remain inside the runtime-support package's control flow and the interceptor
observation paths.

Generated code favors readable repetition over compact indirection. Large graphs
generate longer constructors, but the structure should remain navigable in
ordinary Go IDEs: every component is a named local, and every level is a
contiguous, readable block.

## 9. Deadlines and Configuration

Yama generates no lifecycle configuration and owns no lifecycle policy of its own.

There is no generated configuration structure, no start or shutdown deadline field, and no per-component timeout field. The only deadline is the one carried by the caller's context passed to `Start` and `Stop`. The framework threads that context through the traversal and never lengthens it.

A component that wants a per-component timeout wraps its own `Start`, `Quiesce`, or `Stop`. This is ordinary Go, not a Yama mechanism. Slow-operation and overrun diagnostics are interceptor concerns. Removing lifecycle configuration keeps the framework out of the timeout-policy and configuration business entirely.

## 10. Interceptor Architecture

Interceptors are the primary runtime extension mechanism. They are runtime objects supplied explicitly when the lifecycle implementation is constructed.

Yama uses operation-specific interceptor interfaces:

* start interceptors participate only in `Start`,
* quiesce interceptors participate only in `Quiesce`,
* stop interceptors participate only in `Stop`.

The interceptor interfaces are intentionally not uniform. Yama rejects a single shared interceptor shape. Each interceptor's signature matches the error semantics of the phase it wraps. A start interceptor receives the operation context and a `Starter` next value and returns an `error`, because `Start` can fail. A quiesce interceptor receives the operation context and a `Quiescer` next value and returns nothing. A stop interceptor receives the operation context and a `Stopper` next value and returns nothing, because those phases have nothing actionable to report. A uniform contract would force `Quiesce` and `Stop` to carry an unused error return, or force `Start` to discard the error it must report. Interceptors still observe, suppress, replace, or modify execution by wrapping the `next` value while preserving strong typing.

Lifecycle construction accepts interceptor values through the public, non-generated `WithInterceptors(interceptors ...any) Option` helper. Interceptors attach globally only. There is no per-component scoping, no component names, string keys, runtime lookup, or registration API. The generated constructor passes its `Option`s straight through. The runtime-support package filters the supplied values by operation-specific interceptor interface and builds the chains.

Separate chains are built for start, quiesce, and stop. Each operation chain is, from outermost inward:

1. for quiesce and stop, a framework-owned gate that drops a component whose `Start` failed,
2. interceptors supplied via `WithInterceptors` that implement the operation-specific interceptor interface, in registration order,
3. the built-in overrun interceptor,
4. the component's own lifecycle method.

Application interceptors run in registration order among themselves, bracketed by Yama's own two links: the gate decides whether the component takes part in the pass at all, and the overrun interceptor sits directly around the component's own method. ADR-005 records why each sits where it does and what each position costs. The lifecycle manager does not reorder, prioritize, or rediscover application interceptors.

Chain construction happens once, during lifecycle construction, and the same three chains are reused for every component. Lifecycle execution calls the prebuilt chains. Chain code remains strongly typed and operation-specific. It does not use operation enums, string dispatch, reflection, or plugin discovery.

Interceptors may observe execution, modify context, suppress execution, invoke the next element conditionally, alter returned outcomes, and provide diagnostics. Optional participation, environment policy, logging, metrics, tracing, and telemetry are interceptor responsibilities rather than lifecycle manager responsibilities.

Interceptors require every component to be invoked through a chain, so the wrapper layer is universal: every component is wrapped whether or not an application interceptor is attached to it. Wrapping is not opt-in. The observational overrun logging relies on this same universal wrapping: a built-in, Yama-authored interceptor is attached to every component's chain, and it is what detects and logs a caller's context-deadline overrun. Universal wrapping is what gives that mechanism per-component attribution. This built-in interceptor is an internal implementation detail. Applications neither write nor register it, and it adds no public API.

## 11. Error Handling Architecture

Yama exposes a single lifecycle-level error:

* `ErrStartFailed`

Startup returns `ErrStartFailed` if startup cannot complete successfully. `Stop` returns nothing. There is no recovery from shutdown, so there is nothing to return, and there is no `ErrStopFailed`.

Component errors are not returned to the lifecycle caller. Component identity, duration, deadline-overrun detail, and original component errors belong in interceptors and observability integrations.

Startup-failure cleanup does not change the public startup error. If startup fails, the same internal shutdown sequence (the quiesce pass, then teardown) runs over the successfully started components, and then `ErrStartFailed` is returned. Shutdown produces no error, so cleanup outcomes are observable through interceptors but never change the returned error.

A `Lifecycle` is in one of three states, and `Start` and `Stop` are serialized against each other:

| State | `Start` | `Stop` |
| --- | --- | --- |
| Stopped — never started, or fully stopped | runs the levels | no-op |
| Started | no-op, returns nil | runs both shutdown passes |
| Spent — a level failed during a start | returns `ErrStartFailed` without re-running | no-op; cleanup already ran |

A start becomes spent the moment any level fails, whether or not any component had come up first. One failure does *not* spend a lifecycle: the pre-flight check. `Start` observes the caller's context before running any level. An already-canceled or already-expired context returns `ErrStartFailed` having started and torn down nothing, leaving the lifecycle startable under a live context. Both operations are idempotent, so `Stop` may be called unconditionally after a failed `Start`. `Start` after a completed `Stop` runs the levels again. ADR-003 calls this the unpromised restart. ADR-006 records the reasoning.

Shutdown runs the quiesce pass and the teardown pass in dependency order, to completion, returning nothing. The framework waits for each component rather than returning early, so a hung component stalls everything after it in the traversal until the orchestrator sends SIGKILL. This is an accepted consequence of never violating reverse-topological ordering.

## 12. Context Propagation Architecture

The caller's context enters `Start` and `Stop`. Operation contexts are derived from it by attaching lifecycle component metadata.

Before invoking interceptors, the per-component wrapper attaches the current lifecycle component to context. This supports diagnostics, logging, metrics, tracing, and telemetry. The access mechanism is part of the framework-defined interceptor contract: interceptor implementations receive the operation context after the component has been attached and can read it with `yama.FromContext[T](ctx)`. This is the only way an interceptor can reach the component, since its `next` argument is the rest of the chain rather than the component. The lifecycle operation does not need separate context metadata, because the operation-specific interceptor method identifies whether the call is Start, Quiesce, or Stop. The context carrier uses unexported keys so components cannot accidentally collide with framework metadata. The accessor exposes the component only. It does not expose graph APIs, generated implementation types, lifecycle plans, or component error details.

Interceptors may replace or wrap the context before invoking the next element in the chain. Component lifecycle methods receive the context produced by component context injection and interceptor processing.

Yama never extends an existing caller deadline and generates no deadline of its own. The caller's context deadline, if any, is the only deadline, and it remains authoritative.

Components receive an ordinary context that may carry a deadline and handle it with standard Go idioms, including detaching with `context.WithoutCancel` or a fresh context for work that must complete regardless of the deadline. The framework introduces no special contract here beyond "you get a context."

On startup failure, no further startup levels are scheduled. Yama derives no context of its own to cancel. In-flight components in the failing level continue under the caller's context and are awaited.

## 13. Generated Naming Strategy

Because the generated file emits no types and no methods (§8), Yama's naming
strategy is mostly a matter of what it does *not* have to name. There is no
generated level type, no generated lifecycle implementation type, and no
generated wrapper or chain type. Those roles all live in the runtime-support
package, where they are that package's own private identifiers rather than
generated ones.

Four kinds of identifier appear in the generated file, and only the last two are
Yama's to derive:

* **Constructor names**, which the application chooses by declaring a stub
  (ADR-011). Yama derives no constructor name, so there is no derivation rule to
  specify. Two stubs sharing a name is an ordinary Go redeclaration error in the
  application's own file.
* **Component locals**, which are the injector-local variable names Google Wire
  already emitted, reproduced by the re-emission of the injector body.
* **Cleanup locals**. Google Wire names a provider's cleanup positionally, which
  says nothing about the value it releases, so the re-emitted body rebinds each
  cleanup to a name derived from that value (ADR-008, ADR-013).
* **Import names**, which the constructor's tail uses to reach the public
  package and the runtime-support package. They share one scope with the
  component locals Wire chose, so a collision between the two is possible, and
  the import is the name that gives way (ADR-013).

Yama derives one further identifier that no committed file carries: the name of
the injector it derives from each stub, which lives only in the transient
derived-injector file and in Wire's transient output. That name is derived from
the stub's, in a reserved namespace, so each stub maps to one injector in Wire's
output and an application's own injectors are never mistaken for Yama's
(ADR-011, ADR-013).

A derived identifier must be:

* deterministic across equivalent inputs,
* stable enough for review,
* unique within the generated package.

A name already taken in the scope it must be unique in is escaped with a numeric
suffix, matching the convention Google Wire's own output already carries
(ADR-013).

Uniqueness is a requirement on the emitted file, not a promise that every input
can satisfy it. An input that cannot be emitted as a compilable file fails
generation at build time instead (§4).

Yama derives no component name for the public API either: `yama.FromContext`
yields the component itself, and a component that wants a printable identity
implements `fmt.Stringer`. Nothing an application observes at runtime is named by
Yama.

Public framework API names remain limited to the ADR-defined lifecycle
interfaces, capability interfaces, interceptor interfaces, `Option` and its
constructors, and the lifecycle-level error. The runtime-support package's
exported builder surface is reachable by generated code but is not part of that
public API (ADR-007, ADR-010).

## 14. Generated Artifact Layout

Generation produces one committed file in the target application package: Yama's `lifecycle_gen.go`. One `go:generate` directive invokes Yama, which derives a Google Wire injector from each lifecycle stub, runs Google Wire over it, parses the generated injector, and emits the lifecycle file. Google Wire's generator is pinned as a Go tool dependency in the application's `go.mod` and invoked as `go tool wire gen`, so generation is reproducible and needs no assumption about `wire` being on `PATH`. A build tag must not exclude the file that holds the `go:generate` directive.

The package carries one hand-authored, build-tagged declaration file that
generation reads: the lifecycle stub file, behind `//go:build yamainject`,
declaring each constructor's name and signature and the providers that build its
graph (ADR-011). An application may also carry a `wire.go` behind
`//go:build wireinject` for injectors it wants for its own purposes. Yama neither
reads nor requires one.

Generation writes two transient files into the package directory and removes
both: the derived-injector file, behind `//go:build wireinject`, and Google
Wire's `wire_gen.go`. Neither is committed. Removal is non-destructive, so a file
of either name that Yama did not create is preserved and restored (ADR-008,
ADR-011).

The derived-injector file carries two declarations per lifecycle stub. One is
the injector Google Wire generates a body for. The other declares the
constructor's own name and whole signature, over a body Google Wire does not
read as a template. Google Wire copies that second declaration into its own
output, which is where Yama's later load of that output resolves the constructor
(ADR-011).

`lifecycle_gen.go` carries `//go:build !yamainject`, which keeps the emitted
constructor out of the load that reads the stubs, since those declare the same
function. It states no other tool's build condition (ADR-011).

A run also moves the committed `lifecycle_gen.go` aside, beside the two
transient files, and puts it back afterward. The committed file and the
derived-injector file declare the same constructors, and the scope keeps the two
apart. An application file that calls a lifecycle constructor therefore compiles
during Google Wire's run and during Yama's own load of Wire's output, in the
constructor's own package and in a sibling package alike.

The scope covers a second hazard. Google Wire type-checks the whole package, and
so does Yama's load of Wire's output. A committed file left stale by a provider
rename would otherwise fail the step that produces its replacement (ADR-011).

Google Wire may be unable to build a stub's declared result. This can happen because
providers do not reach it, or because a dependency cycle exists among them. Either case
is a generation failure, reported against the stub rather than against the derived
injector the application never sees.

Google Wire's `wire_gen.go` is a transient intermediate. Google Wire writes it into the package directory. Yama parses it and then removes it, and it is not committed. Removal is non-destructive: Yama removes only a `wire_gen.go` it generated, and a pre-existing one is moved aside before generation and restored afterward, so generation never overwrites or deletes a file Yama does not own. Because `wire_gen.go` is absent from the built package, the application constructs and runs through the generated lifecycle constructor, not through Google Wire's injector.

A CI check regenerates `lifecycle_gen.go` and diffs it against the committed copy to catch drift. A change in Yama's parser, analysis, or emitter changes the lifecycle file's content.

The lifecycle file contains:

* a generated-file header recording provenance (`// Code generated by Yama. DO NOT EDIT.`),
* the build constraint excluding it from the two injection builds,
* package declaration,
* imports required by generated lifecycle code, including the Yama runtime-support package,
* one lifecycle constructor per stub, each re-emitting its derived injector's
  construction body and then declaring its levels through the builder.

Nothing else is emitted. There are no generated types and no generated methods:
level representation and execution, chain construction, per-component wrapping,
boundary placement, cleanup wrapping, and the built-in overrun interceptor all
live in the runtime-support package the lifecycle file imports (ADR-010).

Generated artifacts are implementation details. Applications may inspect them for debugging and review, but they should not depend on private generated type names or helper method names.

## 15. Startup Architecture

`Start(ctx)` executes the declared levels in the order the generated constructor
declared them, which is dependency order.

A level is driven exactly as a component is: it is a `Starter`, a `Quiescer`, and
a `Stopper`, so the lifecycle treats it uniformly, and a member that takes no part
in a pass is a no-op for that pass rather than absent from it. Inside the level,
all members run concurrently and the level waits for all of them. A cleanup
contributes no start work. Each component invocation passes through:

1. component-identity context injection,
2. the prebuilt start interceptor chain,
3. the component's `Start` method.

A `Start` is expected to return once the component's start side effects have begun. Construction already produced an inert, valid value. A component whose start would otherwise block (for example, `http.Server.ListenAndServe` or `grpc.Server.Serve`) is responsible for launching the blocking call in its own goroutine and returning, routing the error wherever it needs to go. This is ordinary Go. The framework provides no helper for it.

Each component's start outcome is recorded as its `Start` returns, and the
traversal proceeds to the next level once every member of the active level has
settled.

Startup is fail-fast across levels. If any component in the active level fails, no further startup level is scheduled. The framework waits for in-flight operations in the active level to settle on their own terms, then runs the normal shutdown sequence (quiesce pass, then teardown) over the levels reached so far, and returns `ErrStartFailed`. A sibling's failure does not cancel the components running beside it. Within the failing level, the components that did come up are torn down and the ones that failed are gated out.

Components in later levels are never started after a startup failure, and those levels take no part in the cleanup that follows.

## 16. Quiesce Architecture

Quiesce is invoked internally by shutdown processing, as the first pass of `Stop`. Applications do not call quiesce directly, and it is not exposed on `Lifecycle`.

The quiesce pass walks the declared levels back, the same direction as teardown, because a dependency must not quiesce while a dependent might still call into it. Inside each level, quiesce-capable members run concurrently. A member that implements no `Quiescer` is a no-op for the pass. Each component invocation passes through:

1. the caller's context (shared across the whole shutdown),
2. component-identity context injection,
3. the prebuilt quiesce interceptor chain,
4. the component's `Quiesce` method.

`Quiesce` returns no error. It stops accepting new work and blocks until the component's in-flight work completes. The caller's context deadline is observational: the framework keeps waiting for `Quiesce` to return, and the built-in overrun interceptor reports the overrun once it does. A slow component stalls the components that depend on it until it returns or SIGKILL arrives.

Idempotent shutdown is the framework's own guarantee: `Stop` runs the quiesce and teardown passes once, so repeated or overlapping `Stop` calls do not re-trigger them. A component whose own "stop accepting new work" step must fire exactly once uses ordinary `sync.Once`. The framework provides no helper for it.

During startup-failure cleanup, the quiesce pass is scoped to components that successfully started before startup failed.

## 17. Shutdown Architecture

`Stop(ctx)` executes shutdown processing:

1. Run the quiesce pass over successfully started quiesce-capable components in reverse dependency order.
2. Run the teardown pass over successfully started stop-capable components in reverse dependency order.
3. Return nothing.

Both passes run under the caller's context: the single context passed to `Stop`. Its deadline, if any, spans the whole sequence. The quiesce pass completes before any teardown begins.

Levels are declared once, in dependency order, by the generated constructor. Both shutdown passes walk that one declaration back rather than reading a second, reversed structure. A dependent stops before the dependency it relies on. Independent members in the same level stop concurrently inside the level. A member with no stop work is a no-op for the pass. A Google Wire cleanup runs as the teardown of the value it cleans up: ahead of a lifecycle-capable component's own `Stop`, or alone as a dependency-only component's entire `Stop`. Cleanups do not pass through interceptor chains. They are not gated on the outcome of a start: a component whose `Start` failed still releases what its provider acquired.

Each stop invocation passes through:

1. the caller's context (shared across the whole shutdown),
2. component-identity context injection,
3. the prebuilt stop interceptor chain,
4. the component's `Stop` method.

`Stop` returns no error. The traversal runs to completion in dependency order. The framework waits for each component rather than returning early, so a hung component stalls everything after it until SIGKILL.

## 18. Boundary Components

Yama supports two boundary registration points that sit outside the construction
DAG: a **begin** boundary and an **end** boundary. Boundary components are peers of the
graph components. A boundary component participates in the lifecycle through whichever of
`Starter`, `Quiescer`, and `Stopper` it implements, exactly as a graph component does.
Registering a component in a boundary does not change what it does. It controls only its
execution order relative to the graph.

The lifecycle runs three passes: the Start pass in dependency order, and the
Quiesce and Stop passes in reverse dependency order. Boundaries are modeled as dependency
extremes of the whole graph: a begin component behaves as a base every graph component
depends on, and an end component behaves as a top that depends on every graph component. So
the same ordering rule that governs the graph governs them. A begin component therefore starts
before every graph component and quiesces and stops *after* every graph component. An end
component starts after every graph component and quiesces and stops *before* every graph
component. Shutdown is the exact reverse of startup. A boundary component joins a pass only
when it implements that pass's interface:

```text
Start pass     begin Starters   → graph Starters  → end Starters
Quiesce pass   end Quiescers    → graph Quiescers → begin Quiescers
Stop pass      end Stoppers     → graph Stoppers  → begin Stoppers
```

A boundary registration expresses one thing: this component sits at a dependency extreme of
the graph, a base that comes up first and goes down last, or a top that comes up last and
goes down first. It exists so that position can be stated directly instead of by wiring
dependency edges against every graph root and revising them whenever the set of roots
changes.

Each boundary is a flat, unordered set. Components in the same set have no ordering
relationship and may execute concurrently. Yama makes no ordering guarantee among
them. A boundary component has no dependency relationship to any particular graph component.
Anything that needs an ordering relative to specific components has a real dependency
relationship and belongs in the construction graph, not in a boundary set.

A boundary component's failure is handled exactly like a graph component's, because
boundary placement changes execution order only, never failure handling: a Start error
or panic is fail-fast and surfaces as `ErrStartFailed`, the same as a failing graph
`Starter`. A Quiesce or Stop error or panic is recovered so the pass runs to completion
and returns nothing, the same as for a graph component. Like graph components, boundary
components are wrapped, so their failures and overruns are observable through
interceptors. A caller that wants a boundary component's failure isolated from the pass,
treated as optional rather than required, wraps that component so it recovers its own
panics and swallows its own errors before Yama ever sees them. Yama itself makes no such
accommodation.

In each pass, boundary components run under the same caller context as the graph components
and share its deadline. Yama gives a boundary component no budget of its own and does
not preempt it, so a slow begin component consumes budget that the rest of the pass would
otherwise have. This is a documented consequence, not a mitigated one.

Because boundary components are part of the passes rather than a separate step, they
bracket those passes wherever they run. This includes the internal startup-failure
cleanup, which reuses the Quiesce and Stop passes over successfully started
components. There is no separate boundary execution path.

Boundary components are supplied as runtime objects when the lifecycle value is
constructed, using the `WithBeginComponents` and `WithEndComponents` options alongside
`WithInterceptors`. They are not derived from the Wire graph and are not lifecycle
graph components, so they never appear in the generated file: a generated
constructor declares only the graph's own levels.

Their dependency-extreme position is realized by placing each boundary set in a
level of its own: the begin set as the level before every graph level, the end
set as the level after. The runtime-support package adds these levels around the levels
generated code declares (ADR-010). The begin and end sets are therefore walked by
the same forward and backward traversals, gated by the same start-outcome rule,
and wrapped through the same interceptor chains as any graph level, which is what
gives them the ordering rule ADR-009 specifies. A boundary set with no components
is an empty level and does nothing.

Two cases illustrate the boundaries without defining them:

* Telemetry, or another base service every component uses, is a begin component. It
  starts before the graph, and because shutdown reverses startup, its `Quiesce` and
  `Stop` run *after* every graph component. So components can still log, trace, and emit
  metrics while they quiesce and tear down. A begin component outlives everything that
  depends on it. This holds only if it owns its transport. If it depends on a
  Wire-constructed connection or pool, it has a genuine dependency on a graph component and
  belongs in the construction graph, not the begin boundary.
* An in-process readiness flip is an end component. As a `Quiescer`, its `Quiesce` runs
  before every graph component quiesces, so the readiness probe flips to failing before
  the graph drains and the routing layer stops sending new work. As a `Starter`, it starts
  after the whole graph is up, so the process reports ready only once everything behind it
  is running. (See Appendix A for how this relates to the `preStop` hook, which is the
  primary mechanism and is out of Yama's scope.)

## 19. Concurrency Model

Concurrency opportunities are computed during generation.

Startup and shutdown are divided into the explicit levels the generated constructor declares. Components in the same level may execute concurrently because lifecycle analysis determined at generation time that no lifecycle ordering edge exists between them. Levels execute sequentially, and each level is driven through the same capability interfaces its members implement.

The quiesce pass follows the same reverse dependency ordering as teardown. It does not ignore dependency ordering. Independent branches quiesce concurrently, but a component quiesces only after the dependents that rely on it.

Intra-level concurrency uses standard Go synchronization primitives: goroutines, wait groups, atomics. These are private implementation details of the runtime-support package. They coordinate only the operations of the current lifecycle call and do not represent runtime graph state or a runtime execution plan.

Startup is fail-fast across levels: a failed level stops every later level. Within a level, members run to completion and are awaited. A sibling's failure does not cancel them. The quiesce and teardown passes coordinate ordered work that waits for each component to return, so ordering is never violated to reclaim liveness.

## 20. Timeout Handling

Yama generates no deadline and owns no timeout policy. The only deadline is the one carried by the caller's context passed to `Start` and `Stop`. That single context is threaded through the whole traversal. For shutdown, the quiesce pass and teardown pass share it. Generated code never lengthens the caller's deadline.

The deadline is observational. The framework does not return early when it expires. It continues waiting for the component's operation to actually complete, and the built-in overrun interceptor attached to every component reports the overrun, with per-component attribution, once the operation returns. Returning early would let the traversal reach a component's dependencies while that component might still be using them, violating reverse-topological ordering. Preserving ordering is chosen over liveness. External liveness is bounded by the orchestrator's SIGKILL.

Because the report is emitted on return, a component that never returns produces no overrun record at all. The deadline is not watched while the wait is in progress. A hung component is visible as the absence of everything after it in the traversal, not as a log line naming it.

There is no timeout error and no framework-owned remediation. Because one context spans the whole shutdown, a slow quiesce can consume the window and leave teardown little time. This is accepted. Overruns are observable through interceptors, which receive the operation context and observe the operation.

A component that needs a per-component timeout wraps its own `Start`, `Quiesce`, or `Stop`. This is ordinary Go, not a Yama mechanism. Components handle the deadline with ordinary Go idioms. Work that must complete regardless can detach with `context.WithoutCancel` or a fresh context.

This fixes the boundary of Yama's responsibility. Yama guarantees phase ordering and deadline propagation: it runs each phase in the correct dependency order and threads the caller's context, with its deadline, through every component without lengthening it. Honoring that context is the component's responsibility. Yama does not preempt an uncooperative component. A component that ignores its deadline is stalled only by the orchestrator's SIGKILL. This holds for graph components and boundary components alike.

## 21. Observability Architecture

Yama is observability-tool agnostic. It does not expose logger, tracer, meter, health, or readiness APIs.

Observability is implemented through interceptors and lifecycle metadata propagated in context. The component is attached to the operation context before the chain runs, which is enough for an interceptor to associate an observation with a component and an operation without any public graph API.

Interceptors can measure duration, log failures, emit metrics, start traces, record deadline overruns, and record component diagnostics. The lifecycle manager itself only determines lifecycle outcomes.

Yama emits records of its own in exactly three cases, each as a single record through `log/slog`'s package-level default logger:

* a **deadline overrun**, at Warn, from the built-in interceptor attached to every component, with per-component attribution. An application interceptor can observe this for itself by reading the deadline around its own `next` call. The built-in record exists so that per-component attribution is available without one.
* a **skipped component**, at Warn, when the started-gate drops a component whose `Start` failed from a shutdown pass. The gate is outermost, so no interceptor runs for that component and the record is the only signal.
* a **recovered panic**, at Error with the panic value and stack. Recovery happens at the level, above the whole chain: an interceptor sees a panic only if it defers a recover of its own, and a sibling's interceptors never see it at all, since each member panics inside its own goroutine. The record is what reports it by default.

`slog`'s default is deliberately the only channel and is not configurable through Yama (ADR-005, ADR-006).

## 22. Failure Scenarios

Startup component failure:

* stop scheduling startup levels,
* wait for in-flight operations in the active level to settle uninterrupted,
* quiesce successfully started components,
* stop successfully started components,
* return `ErrStartFailed`.

Caller's start context already canceled or expired when `Start` is called:

* rejected before any level runs,
* nothing started, nothing torn down, the lifecycle stays startable (§11),
* exposed publicly as `ErrStartFailed`.

Caller's start context deadline exceeded mid-traversal:

* handled as startup component failure, so the lifecycle is spent,
* exposed publicly as `ErrStartFailed`,
* details available only through interceptors and observability.

Quiesce component that does not return:

* the framework keeps waiting,
* it does not return early,
* once it returns, its context-deadline overrun is reported with per-component attribution,
* dependencies protected by the component do not quiesce until it returns,
* nothing is returned to the caller.

Stop component that does not return:

* the framework keeps waiting,
* the traversal does not proceed past it,
* no overrun is reported, because the report is emitted on return,
* nothing is returned to the caller.

Hung component:

* a component that never returns stalls everything after it in the traversal until the orchestrator sends SIGKILL,
* this is intentional and follows from preserving reverse-topological ordering.

Cleanup after startup failure:

* run the quiesce pass and teardown pass for successfully started components,
* return `ErrStartFailed`,
* expose cleanup diagnostics through interceptors.

Caller context cancellation:

* propagate cancellation through the derived operation contexts,
* during startup, treat resulting failures as startup failure and return `ErrStartFailed`,
* during shutdown, the passes still run to completion in order and return nothing.

## 23. Performance Considerations

Yama moves expensive lifecycle work to generation time:

* running `wire gen`,
* parsing the generated injector,
* lifecycle capability analysis,
* topological level computation from the injector's statement order,
* chain shape generation.

Runtime avoids graph traversal, sorting, discovery, registration, reflection, and plan interpretation.

Runtime overhead consists primarily of:

* walking the declared levels,
* launching each level's members concurrently,
* invoking interceptor chains,
* tracking which components started successfully,
* aggregating lifecycle-level success or failure state internally.

Generated code may be larger for large dependency graphs. This is an accepted trade-off for readability, determinism, and ordinary Go debugging.

## 24. Testing Strategy

Testing covers the generator, generated code shape, and runtime behavior of generated lifecycle implementations.

Generator tests should use real source-file fixtures containing Google Wire injector inputs, run `wire gen`, and assert that the AST walk of the resulting `wire_gen.go` derives the correct ordering. Fixtures should cover provider functions, provider sets, bindings, values, struct providers, field providers, injector declarations, transitive ordering through dependency-only components, and cleanup functions. The tests assert that lifecycle analysis produces the correct level list and classifies each member's teardown form, that a value which both returns a cleanup function and implements `Stopper` has the cleanup folded into its teardown ahead of that value's own `Stop`, and that a value which returns a cleanup but implements no capability occupies a level on the strength of the cleanup alone. These tests operate on the generated injector and generated source, not on a private runtime graph API.

Generated-code tests should compile generated packages and exercise the public lifecycle surface. They should verify startup ordering, shutdown ordering, quiesce-before-teardown behavior, reverse-topological quiesce ordering, concurrent independent branches, startup fail-fast behavior, startup-failure cleanup, that shutdown returns nothing and runs to completion, observational-deadline overrun logging, and that only `ErrStartFailed` is returned. Golden-file tests should cover generated source shape for representative graphs so readability, identifier stability, and level declaration remain reviewable.

Interceptor tests should verify separate operation chains, the non-uniform signatures, registration-order execution, context modification, behavior suppression, and outcome modification.

Error tests should verify that callers receive only `ErrStartFailed`, that `Stop` returns nothing, that repeated and out-of-state `Start`/`Stop` calls behave as §11's table says, and that component errors reach only interceptors. The three records Yama emits itself (§21) are asserted by installing a capturing `slog` handler.

Drift tests should regenerate `lifecycle_gen.go` and diff it against the committed copy, so a change in Google Wire's generated output shape fails visibly at the drift boundary rather than at runtime.

## 25. Rejected Alternatives

Runtime lifecycle plans are rejected because generated executable Go code is the authoritative lifecycle representation.

Runtime graph construction and runtime graph introspection are rejected because the dependency ordering is a generation-time artifact derived from Google Wire's generated injector.

Adapting or forking Google Wire's generation internals is rejected because Google Wire's graph-construction and solver logic are declared in `internal/` packages that external code cannot import, and depending on them would require a fork to maintain. Yama runs `wire gen` and walks the generated injector instead, coupling to Google Wire's output shape rather than to its internals. Google Wire is archived, so that shape is fixed.

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

Yama runs graceful shutdown when the process receives SIGTERM. It does not encode any orchestrator-specific behavior. The guidance below is documentation, not a mechanism inside the library.

**The readiness-to-routing gap belongs to a `preStop` hook, not to Yama.** When a pod's readiness probe flips to failing, there is a delay before the load balancer actually stops routing to it. That gap is covered by a Kubernetes `preStop` hook, which delays SIGTERM, not by an artificial sleep inside the library. Yama's `Quiesce` intentionally adds no such delay, because doing so would encode an orchestrator-specific assumption into a general-purpose library.

**The shutdown budget must fit inside `terminationGracePeriodSeconds`.** After SIGTERM, Kubernetes sends SIGKILL once the grace period elapses. The deadline on the context the caller passes to `Stop`, plus any `preStop` delay, should fit within `terminationGracePeriodSeconds`, because SIGKILL, not the observational deadline, is what ultimately bounds shutdown.

**An in-process readiness flip is an end boundary component, not a graph component.** The primary mechanism for the readiness-to-routing gap is the `preStop` hook above, which fires before SIGTERM and is out of Yama's scope. The total shutdown budget, including any `preStop` delay, must fit within `terminationGracePeriodSeconds`. An application may additionally want to flip readiness from inside the process. If so, that flip should run before the graph quiesces, at the very start of shutdown, which makes it an end boundary component (see Boundary Components): a `Quiescer` registered in the end boundary, whose `Quiesce` runs before every graph component's, rather than a `Quiescer` wired into the construction graph. Modeling it as a graph component would require wiring dependency edges only to force it to the front of the quiesce pass. The end boundary expresses that position directly.

## Appendix B. Long-Lived Work

Yama cannot manufacture time that is not there. The observational deadline waits for a component, but the orchestrator's SIGKILL can still land mid-operation regardless of how the component handles its context.

Work that must not be lost therefore has to be crash-safe and resumable at the storage layer (write-ahead logging, an outbox, or atomic and replayable writes), so that a process killed mid-operation can recover on restart. This is application-level guidance, not a Yama feature. A component may detach from the shutdown deadline with `context.WithoutCancel` to finish an in-flight unit of work, but it cannot rely on always being allowed to finish, so durability must not depend on shutdown completing.

## Appendix C. Public API Reference

ADR-007 records the decision to keep Yama's public API minimal, and the reasoning behind it. This appendix is the authoritative enumeration of that surface: `package yama`'s complete set of exported symbols. It is the document to update whenever that surface changes. ADR-007 argues the shape of each part. It does not carry the catalog.

### Lifecycle Type

```go
type Lifecycle interface {
    Starter
    Stopper
}
```

`Lifecycle` is an interface composed of the capability interfaces below: it starts
and stops the whole graph, so it is the same kind of thing as the components
inside it. Its implementation is private and owned by the runtime-support package.
Applications receive a `Lifecycle` and never implement or construct one. `Starter`
and `Stopper` are themselves public and frozen, so the composition adds no
compatibility commitment beyond those already made.

The generated constructor returns the application and its `Lifecycle` together:

```go
app, lifecycle, err := NewLifecycle(WithInterceptors(i1, i2), WithBeginComponents(c1), WithEndComponents(c2))
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
func FromContext[T any](ctx context.Context) (T, bool)
```

The accessor yields the lifecycle component itself. `T` is the component's concrete type. Because interceptors attach globally (ADR-005), callers type-switch on the yielded value with `T` as `any`. `T` is unconstrained because Go cannot express "implements at least one of `Starter`, `Quiescer`, `Stopper`." This is the same limitation that makes `WithBeginComponents` take `any`.

Yama derives and exposes no component name. A component that wants a printable identity implements `fmt.Stringer`. `%T` yields its type otherwise.

### Errors

```go
var ErrStartFailed error
```

### Options

```go
type Option interface {
    Apply(*bridge.Config)
}
```

`Option` is the construction-time input to the generated constructor. It is
exported so generated code and callers can name it, but it is **sealed**:
implementing it outside Yama means naming `*bridge.Config`, which Go's `internal/`
rule forbids outside this module. The `Option` constructors are therefore exactly
`WithBeginComponents`, `WithEndComponents`, and `WithInterceptors`. A caller
cannot introduce a fourth.

### Helpers

```go
func WithBeginComponents(components ...any) Option // base-extreme components: start before the graph, tear down after it
func WithEndComponents(components ...any) Option   // top-extreme components: start after the graph, tear down before it
func WithInterceptors(interceptors ...any) Option // attach interceptors globally
func RunUntilSignal(lc Lifecycle, signals ...os.Signal) error // Start, wait for a signal, then Stop
```

`WithBeginComponents`, `WithEndComponents`, and `WithInterceptors` take `any` because Go cannot express a union of method-bearing interfaces: neither `Starter | Quiescer | Stopper` for components, nor the three interceptor interfaces. Yama detects each value's capabilities by type assertion. All are variadic and may be called more than once. Registered values accumulate in call order.

### Explicitly Not Public

The exported symbols of the Yama-owned runtime-support package (ADR-010) are not part of this surface, even though they are exported so generated code can reach them: `NewLifecycleBuilder`, and `LifecycleBuilder` with `NextLevel`, `WithComponents`, `WithCleanableComponent`, `WithCleanup`, and `Build`. The generated constructor in the application's own package is not part of it either: it lives in the application, and the application names it (ADR-011). Applications should not depend on anything outside the list above.
