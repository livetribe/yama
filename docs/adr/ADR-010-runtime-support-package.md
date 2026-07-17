# ADR-010: Runtime-Support Package and the Generated/Shared Split

## Status

Accepted

## Context

Yama emits `lifecycle_gen.go` into the target application's package (ADR-008,
Architecture §14). That generated file needs execution machinery to do its work:
interceptor chain construction, a per-node wrapper that attaches component identity
and threads the caller's context, a fail-fast intra-level executor, a boundary
runner, the `cleanupAdapter` that wraps Google Wire cleanup functions as `Stopper`,
and the built-in per-node overrun interceptor.

Two facts constrain where that machinery can live:

* Because `lifecycle_gen.go` is compiled as part of the *application* package, it
  cannot call unexported identifiers in `package yama`. Go's visibility rules make
  that a hard compile-time boundary, not a style preference.
* That machinery is **generic**. It is identical in every application and does not
  depend on the shape of any particular dependency graph. What *does* depend on the
  graph — which participants sit in which level, and in what order the levels run —
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

Only the **graph-specific** parts are generated inline into `lifecycle_gen.go`:

* the private level structs that name concrete participants,
* their Start/Quiesce/Stop ordering methods,
* the `YamaInterceptors` input,
* the `NewLifecycle` constructor.

The runtime-support package holds the **graph-independent** parts:

* interceptor chain construction,
* the per-node wrapper (component-identity attachment, context threading),
* the fail-fast intra-level executor and the ordered quiesce/stop passes,
* the boundary runner,
* `cleanupAdapter`,
* the built-in per-node overrun interceptor.

`package yama` retains only the stable public API of ADR-007. The runtime-support
package is a distinct package precisely so that the ADR-007 surface stays minimal
and its API-surface check stays clean, while generated code still has exported
symbols to call.

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

### ADR-004 intent is preserved

ADR-004 requires that execution ordering be readable in generated code rather than
hidden in a runtime engine. Ordering lives in the generated level structs and their
methods, which stay inline. The runtime-support package holds only mechanical
plumbing, not ordering, so the generated file remains the authoritative, readable
description of *what runs in what order*.

### Established Go pattern

Generated Go code routinely calls into an exported, convention-internal runtime
package (for example protobuf's `protoimpl`, ent, and gRPC's generated stubs). This
decision follows that well-worn pattern.

## Consequences

### Positive

* Generated application-package code compiles against exported symbols.
* The stable `package yama` public API stays exactly the ADR-007 surface.
* The plumbing is written and tested once, not duplicated per generated file.
* Execution ordering remains visible in the generated code (ADR-004 holds).

### Negative

* There is a second Yama-owned package to maintain.
* The runtime-support package exposes exported symbols that are **not** covered by
  the stable-API compatibility promise; this must be documented so applications do
  not depend on it directly.

### Accepted Trade-Off and Reversibility

This split is the well-justified default, not an irreversible commitment. Because
the runtime-support surface is demarcated and explicitly non-stable, a later change
— for example emitting the plumbing inline instead — is contained. The
expensive-to-change artifact is the *generated code shape*, not which package the
helpers occupy. The point at which the real generated shape first becomes concrete
is the natural moment to re-confirm this decision before it is baked into the
emitter.

## Rejected Alternatives

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
* Introduce a runtime lifecycle plan or interpreter (ADR-004 still holds; the
  runtime-support package executes generated methods, it does not interpret a plan).
* Change the lifecycle model, error model, or interceptor model.
