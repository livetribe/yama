# ADR-010: Runtime-Support Package and the Generated/Shared Split

## Status

Accepted

## Context

Yama emits `lifecycle_gen.go` into the target application's package (ADR-008,
Architecture §14). Around that generated file sits execution machinery: something
to declare a level against, level representation and the concurrent intra-level
executor, the ordered walk over levels, interceptor chain construction, a
per-component wrapper that attaches component identity and threads the caller's
context, per-component started state, placement of the ADR-009 boundary sets at
the extremes of the walk, the wrappers that give a Google Wire cleanup `Stopper`
behavior — paired with the component it cleans up, or standing alone at a
dependency-only component's position — and the built-in per-component overrun
interceptor.

Two facts constrain where that machinery can live:

* Because `lifecycle_gen.go` is compiled as part of the *application* package, it
  cannot call unexported identifiers in `package yama`. Go's visibility rules make
  that a hard compile-time boundary, not a style preference.
* That machinery is **generic**. It is identical in every application and does not
  depend on the shape of any particular dependency graph. What *does* depend on the
  graph — which components sit in which level, and in what order the levels run —
  is a small, separate concern.

So Yama must decide where the generic machinery lives such that generated
application-package code can call it, without expanding the stable public API
(ADR-007) or hiding execution ordering from the generated code (ADR-004).

## Decision

The generic execution machinery lives in a dedicated, Yama-owned **runtime-support
package** (a sibling of `package yama`, for example `l7e.io/yama/v2/rt`) whose
symbols are **exported so generated code can import and call them**, and which is
**documented as "called by generated code; not part of the stable ADR-007 public
API."**

`package yama` retains only the stable public API of ADR-007. The runtime-support
package is a distinct package precisely so that the ADR-007 surface stays minimal
and its API-surface check stays clean, while generated code still has exported
symbols to call.

### What is generated

`lifecycle_gen.go` contains the provenance header, the build constraint, the
package clause, the imports, and **one constructor per opted-in injector
(ADR-011), and nothing else**. It emits no types and no methods. Each
constructor:

1. re-emits its injector's construction body, so every value the graph builds —
   including the injector-locals Wire's own signature does not return — is in
   scope (ADR-008);
2. opens a builder, declares each level in dependency order, and names that
   level's members;
3. seals the builder and returns the application beside its `Lifecycle`.

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

The three `With…` forms are the teardown forms a member can take, which is the
one thing about a member the generator must decide and the runtime cannot: the
value alone, the value paired with the Google Wire cleanup its provider returned,
or — for a value that implements no capability — that cleanup standing alone at
the value's position.

Interceptors and boundary components are supplied through the public,
non-generated `WithInterceptors` (ADR-005), `WithBeginComponents`, and
`WithEndComponents` (ADR-009) `Option`s rather than a generated input, so the
generated constructor passes its options straight through without inspecting
them.

### What the runtime-support package holds

* the level-declaring builder that generated code calls,
* the representation of a level and the ordered walk over the levels — forward to
  start, backward for the quiesce and teardown passes,
* the intra-level executor, which runs a level's members concurrently and waits
  for all of them,
* interceptor chain construction and the per-component wrapper (component-identity
  attachment, context threading),
* the per-component started state and the gates that keep a component whose start
  failed out of the shutdown passes,
* boundary placement: the begin set opens as the first level and the end set is
  appended as the last, so generated code declares only the graph's own levels and
  no application's generated file carries a boundary that was supplied at its call
  site,
* the cleanup wrappers (paired-with-component and standalone),
* the built-in per-component overrun interceptor.

## Rationale

### Go visibility forces an exported home

Generated application-package code cannot reach unexported `package yama` helpers.
The machinery it calls must therefore be exported from *some* importable package.
Making it a separate, explicitly non-stable package satisfies that requirement
without enlarging the stable public API.

### The machinery is generic, so sharing beats regenerating

None of the plumbing varies by graph. Emitting it inline would copy the same code
into every application's `lifecycle_gen.go` (and every golden fixture), adding
volume and a drift risk between a reference implementation and the emitted copies,
for no benefit. Placing it in a shared package removes the duplication and lets it
be unit-tested directly.

### A call chain carries the graph as well as generated types would

Everything a generated level type would hold — which components are in the level,
and where the level sits in the order — the builder chain states literally, in the
same file, in the same order. What a type adds on top is a declaration, a
construction site, three ordering methods, and the obligation to name all of it
uniquely.

### Emitting no types is what keeps generated naming small

Naming levels would be the largest generated-identifier surface, and it buys
nothing an application observes. A `yamaLevelNNN` per level needs a numbering that
stays stable across regeneration under review, and a namespacing rule for the case
where two injectors in one package have different graphs. Emitting no types
removes both, and leaves the generated file with the smallest naming problem it
could have: the constructor names are the application's (ADR-011), the component
locals are Wire's, and the only identifiers Yama derives are the cleanup locals
(ADR-008).

### The constructor is the graph-specific artifact either way

Because the lifecycle needs values the injector's signature does not return, the
constructor must re-emit the injector body rather than call it (ADR-008). Given
that the constructor exists and already carries the graph, generated level types
would only restate its tail in a second form.

### Execution ordering stays in generated code

ADR-004 requires that execution ordering be readable in generated code rather than
hidden in a runtime engine. Level membership and level order are literal statements
in the generated constructor, so the generated file remains the authoritative,
readable description of *what runs in what order*. ADR-004's *The Ordered Level
List* settles the companion question — why holding those levels in an ordered list
at runtime is not the plan that ADR rejects — and this decision adopts that
reasoning rather than restating it.

### Established Go pattern

Generated Go code routinely calls into an exported, convention-internal runtime
package (for example protobuf's `protoimpl`, ent, and gRPC's generated stubs). This
decision follows that well-worn pattern.

## Consequences

### Positive

* Generated application-package code compiles against exported symbols.
* The stable `package yama` public API stays exactly the ADR-007 surface.
* The plumbing is written and tested once, not duplicated per generated file.
* Execution ordering remains visible in the generated code.
* The generated file has no generated type names, so the naming rules and
  collision handling that generated level types would require do not exist.

### Negative

* There is a second Yama-owned package to maintain.
* The runtime-support package exposes exported symbols that are **not** covered by
  the stable-API compatibility promise; this must be documented so applications do
  not depend on it directly.
* There is no generated method per level to set a breakpoint on; ordering is read
  from the constructor's sequence of level declarations rather than stepped
  through as generated code. ADR-004 records why that is an acceptable reading of
  the debuggability it asks for.
* Because level execution is shared rather than emitted, a behavioral change to it
  reaches every application on a dependency bump rather than on regeneration. This
  is the same trade the split makes for chain construction and wrapping, applied to
  level execution as well.

### Accepted Trade-Off and Reversibility

This split is the well-justified default, not an irreversible commitment. Because
the runtime-support surface is demarcated and explicitly non-stable, a later change
— for example emitting the plumbing inline instead — is contained. The
expensive-to-change artifact is the *generated code shape*, not which package the
helpers occupy, and a builder call chain is a narrow, mechanical shape to
re-target.

## Rejected Alternatives

### Generate a private level type per level, with ordering methods

Rejected on the two Rationale grounds above: a type per level is a more expensive
way to state what the call chain already states, and level types would be the
project's only substantial generated-identifier namespace, with the numbering and
per-injector namespacing rules that implies.

What the alternative would buy is a generated stack frame per level to breakpoint
on. That is a genuine loss, recorded under Consequences. It does not outweigh a
per-graph family of types and the naming rules they would need.

### Emit the plumbing inline into `lifecycle_gen.go`

Rejected as the default because it duplicates identical, graph-independent code into
every generated file and every golden fixture, and creates a drift risk between the
reference implementation and the emitted copies. It remains the natural fallback if
the generated shape later argues for it.

### Export the machinery from `package yama` itself

Rejected because it enlarges the stable public API surface that ADR-007 works to
keep minimal, and muddies the API-surface check. A separate package keeps the stable
surface pristine.

### Generate application-package code that calls unexported `package yama` helpers

Rejected because it does not compile: Go visibility rules prevent application-package
code from calling another package's unexported identifiers.

## Non-Goals

This decision does not:

* Expand the stable public API of ADR-007 (the runtime-support package is separate
  and explicitly non-stable).
* Introduce a runtime lifecycle plan or interpreter. The runtime-support package
  executes the levels generated code declared, in the order it declared them, and
  interprets no plan (ADR-004).
* Change the lifecycle model, error model, or interceptor model.
