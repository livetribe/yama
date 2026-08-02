# ADR-010: Runtime-Support Package and the Generated/Shared Split

## Status

Accepted

## Context

Yama emits `lifecycle_gen.go` into the target application's package (ADR-008,
Architecture §14). Execution machinery surrounds that generated file. This
machinery includes:

* the level-declaring builder,
* the level representation and the concurrent intra-level executor,
* the ordered walk over levels,
* interceptor chain construction,
* a per-component wrapper that attaches component identity and threads the
  caller's context,
* per-component started state,
* placement of the ADR-009 boundary sets at the extremes of the walk,
* the wrappers that give a Google Wire cleanup `Stopper` behavior, and
* the built-in per-component overrun interceptor.

A `Stopper` wrapper either pairs with the component it cleans up, or stands
alone at a dependency-only component's position.

Two facts constrain where that machinery can live:

* Because `lifecycle_gen.go` is compiled as part of the *application*
  package, it cannot call unexported identifiers in `package yama`. Go's
  visibility rules make that a hard compile-time boundary, not a style
  preference.
* That machinery is **generic**. It is identical in every application and
  does not depend on the shape of any particular dependency graph. The graph
  does determine one thing: which components sit in which level, and in
  what order the levels run. That is a small, separate concern.

Yama must decide where the generic machinery lives. Generated
application-package code must be able to call it. The location must not
expand the stable public API (ADR-007). The location must not hide execution
ordering from the generated code (ADR-004).

## Decision

The generic execution machinery lives in a dedicated, Yama-owned
**runtime-support package** (a sibling of `package yama`, for example
`l7e.io/yama/v2/rt`). Its symbols are **exported so generated code can
import and call them**. Yama documents the package as **"called by generated
code; not part of the stable ADR-007 public API."**

`package yama` retains only the stable public API of ADR-007. The
runtime-support package exists as a distinct package so the ADR-007 surface
stays minimal and its API-surface check stays clean. Even so, generated code
still has exported symbols to call.

### What is generated

`lifecycle_gen.go` contains the provenance header, the build constraint, the
package clause, the imports, and **one constructor per opted-in injector
(ADR-011), and nothing else**. It emits no types and no methods. Each
constructor does three things:

1. It re-emits its injector's construction body. This keeps every value the
   graph builds in scope (ADR-008), including the injector-locals that
   Wire's own signature does not return.
2. It opens a builder, declares each level in dependency order, and names
   that level's members.
3. It seals the builder and returns the application beside its `Lifecycle`.

Level membership and level order are expressed as a **call chain**, not as
types:

```go
b := rt.NewLifecycleBuilder(opts...)
b.NextLevel().
    WithComponents(base1).
    WithCleanup(base2Cleanup).
    WithCleanableComponent(base3, base3Cleanup).
    Add()
b.NextLevel().
    WithComponents(mid2).
    WithComponents(root2).
    Add()
b.NextLevel().
    WithComponents(root3).
    Add()
lc := b.Build()
```

The three `With…` forms are the teardown forms a member can take. Deciding
which form applies to a member is the one thing about a member that the
generator decides and the runtime does not. A member takes one of three
forms:

* the value alone,
* the value paired with the Google Wire cleanup its provider returned, or
* for a value that implements no capability, the cleanup standing alone at
  that value's position.

Interceptors and boundary components are supplied through the public,
non-generated `WithInterceptors` (ADR-005), `WithBeginComponents`, and
`WithEndComponents` (ADR-009) `Option`s. They are not a generated input. The
generated constructor passes its options straight through, without
inspecting them.

### What the runtime-support package holds

* the level-declaring builder that generated code calls,
* the representation of a level and the ordered walk over the levels
  (forward to start, backward for the quiesce and teardown passes),
* the intra-level executor, which runs a level's members concurrently and
  waits for all of them,
* interceptor chain construction and the per-component wrapper
  (component-identity attachment, context threading),
* the per-component started state and the gates that keep a component whose
  start failed out of the shutdown passes,
* boundary placement,
* the cleanup wrappers (paired-with-component and standalone),
* the built-in per-component overrun interceptor.

The begin set opens as the first level. The end set is appended as the last
level. Generated code declares only the graph's own levels. No application's
generated file carries a boundary that was supplied at its call site.

## Rationale

### Go visibility forces an exported home

Generated application-package code cannot reach unexported `package yama`
helpers. The machinery it calls must therefore be exported from *some*
importable package. Making it a separate, explicitly non-stable package
satisfies that requirement without enlarging the stable public API.

### The machinery is generic, so sharing beats regenerating

None of the machinery varies by graph. Emitting it inline would copy the
same code into every application's `lifecycle_gen.go`, and into every golden
fixture. That copying adds volume. It also creates a drift risk between a
reference implementation and the emitted copies, with no offsetting benefit.
Placing the machinery in a shared package removes the duplication. It also
lets the machinery be tested directly, as a unit.

### A call chain carries the graph as well as generated types would

A generated level type would hold two things: which components are in the
level, and where the level sits in the order. The builder chain states both
literally, in the same file, in the same order. A type would add a
declaration, a construction site, three ordering methods, and the obligation
to name all of it uniquely.

### Emitting no types is what keeps generated naming small

Naming levels would be the largest generated-identifier surface. It would
give an application nothing it can observe. A `yamaLevelNNN` per level needs
a numbering that stays stable across regeneration under review. It also
needs a namespacing rule, for the case where two injectors in one package
have different graphs. Emitting no types removes both needs. It leaves the
generated file with the smallest naming problem it could have: the
constructor names are the application's (ADR-011), the component locals are
Wire's, and the only identifiers Yama derives are the cleanup locals
(ADR-008).

### The constructor is the graph-specific artifact either way

Because the lifecycle needs values that the injector's signature does not
return, the constructor must re-emit the injector body rather than call it
(ADR-008). Given that the constructor exists and already carries the graph,
generated level types would only restate its tail in a second form.

### Execution ordering stays in generated code

ADR-004 requires that execution ordering be readable in generated code,
rather than hidden in a runtime engine. Level membership and level order are
literal statements in the generated constructor. The generated file
therefore remains the authoritative, readable description of *what runs in
what order*. ADR-004's *The Ordered Level List* section settles a companion
question: why holding levels in an ordered list at runtime is not the plan
ADR-004 rejects. This decision adopts that reasoning rather than restating
it.

### Established Go pattern

Generated Go code routinely calls into an exported, convention-internal
runtime package (for example protobuf's `protoimpl`, ent, and gRPC's
generated stubs). This decision follows that established pattern.

## Consequences

### Positive

* Generated application-package code compiles against exported symbols.
* The stable `package yama` public API stays exactly the ADR-007 surface.
* The machinery is written and tested once, not duplicated per generated
  file.
* Execution ordering remains visible in the generated code.
* The generated file has no generated type names, so the naming rules and
  collision handling that generated level types would require do not exist.

### Negative

* There is a second Yama-owned package to maintain.
* The runtime-support package exposes exported symbols that are **not**
  covered by the stable-API compatibility promise. Yama must document this,
  so applications do not depend on it directly.
* There is no generated method per level to set a breakpoint on. Ordering is
  read from the constructor's sequence of level declarations, rather than
  stepped through as generated code. ADR-004 records why that is an
  acceptable reading of the debuggability it asks for.
* Because level execution is shared rather than emitted, a behavioral change
  to it reaches every application on a dependency bump rather than on
  regeneration. This is the same trade the split makes for chain
  construction and wrapping, applied to level execution as well.

### Accepted Trade-Off and Reversibility

This split is the well-justified default, not an irreversible commitment. The
runtime-support surface is demarcated and explicitly non-stable, so a later
change is contained. Emitting the machinery inline instead is one example of
such a change. The expensive-to-change artifact is the *generated code
shape*, not which package the helpers occupy. A builder call chain is a
narrow, mechanical shape to re-target.

## Rejected Alternatives

### Generate a private level type per level, with ordering methods

Rejected on the two Rationale grounds above. A type per level is a more
expensive way to state what the call chain already states. Level types
would also be the project's only substantial generated-identifier
namespace, with the numbering and per-injector namespacing rules that
implies.

The alternative would give a generated stack frame per level to breakpoint
on. That is a genuine loss, recorded under Consequences. It does not
outweigh a per-graph family of types and the naming rules they would need.

### Emit the machinery inline into `lifecycle_gen.go`

Rejected as the default. It would duplicate identical, graph-independent
code into every generated file and every golden fixture. It would also
create a drift risk between the reference implementation and the emitted
copies. This alternative remains the natural fallback, if the generated
shape later makes it the better choice.

### Export the machinery from `package yama` itself

Rejected because it enlarges the stable public API surface that ADR-007
works to keep minimal. It also complicates the API-surface check. A
separate package keeps the stable surface clean.

### Generate application-package code that calls unexported `package yama` helpers

Rejected because it does not compile. Go visibility rules prevent
application-package code from calling another package's unexported
identifiers.

## Non-Goals

This decision does not:

* Expand the stable public API of ADR-007 (the runtime-support package is
  separate and explicitly non-stable).
* Introduce a runtime lifecycle plan or interpreter. The runtime-support
  package executes the levels that generated code declared, in the order
  generated code declared them. It interprets no plan (ADR-004).
* Change the lifecycle model, error model, or interceptor model.
