# Yama v2 — Implementation Plan

**Target:** the compile-time lifecycle orchestration framework described in
`docs/PRD.md` and `docs/adr/ADR-001`…`ADR-010`, with the resolved architecture in
`docs/Architecture.md`.

**Audience:** a coding agent executing discrete, independently testable phases.

**How to read it.** Each phase states *what* to build and *how* to verify it, and
points to the ADRs/Architecture for *why*. That prose is for understanding the
task — it is **not** code-comment source material. Do not copy rationale from this
plan, or from an ADR, into a code comment: per `CLAUDE.md`, *why* lives in
`docs/adr/` and comments carry only the bare fact or invariant. Decisions here are
stated as a fact plus a pointer, never as an argument to be transcribed.

---

## 0. Orientation: what is being built, and what already exists

Yama v2 has **three separable artifacts**:

1. **A hand-written runtime library** (`package yama`) — the stable public API:
   the capability interfaces (`Starter`/`Quiescer`/`Stopper`), the interceptor
   interfaces, `ErrStartFailed`, `FromContext`, the boundary options
   (`WithBeginComponents`/`WithEndComponents`), and `RunUntilSignal`.
2. **A Yama-owned runtime-support package** (e.g. `l7e.io/yama/v2/rt`, ADR-010)
   — the generic execution plumbing (chain construction, per-component wrapper,
   fail-fast level executor, boundary runner, cleanup wrapping, built-in overrun
   interceptor) that generated code imports. It is exported (generated
   application-package code must be able to call it) but **not** part of the stable
   ADR-007 public API.
3. **A build-time generator** — runs `wire gen` (`go tool wire`), walks the AST
   of the resulting `wire_gen.go`, computes lifecycle ordering, and emits
   `lifecycle_gen.go` into the application package.

Generated code *targets* the runtime library, so the runtime contracts are built
and frozen first (Phases 1–4), then the generator that emits code against them
(Phases 5–8), then end-to-end wiring and drift protection (Phases 9–10).

> **Pre-existing-state fact (Phase 0):** `yama.go`, `option.go`, and the
> `*_test.go` / example files on `master`/`v2` today are the **v1 signal-watcher**
> (`NewWatcher`, `WithClosers`, `WithTimeout`, `io.Closer` fan-out) — a *different
> product* from v2. Nothing about v1 is reusable. Phase 0 deletes them (v2 is a
> green-field rewrite).

The design decisions that shape this plan live in the canonical docs, not in a
plan-local assumptions list. The authoritative sources: the PRD (`docs/PRD.md`),
the ADRs (`docs/adr/ADR-001`…`ADR-010`), and the architecture
(`docs/Architecture.md`). Two design points settled during planning are worth
naming because several phases lean on them:

- **The runtime-support package (ADR-010).** Generic execution plumbing (chain
  construction, per-component wrapper, fail-fast level executor, boundary runner,
  cleanup wrapping, the built-in overrun interceptor) lives in a Yama-owned sibling
  package (e.g. `rt`) that generated code imports — exported so
  application-package code can call it, but not part of the stable ADR-007 API.
  Only graph-specific code (the `NewLifecycle` constructor, which declares each
  level and its members) is generated inline; interceptors are supplied via the public, non-generated
  `WithInterceptors` option (ADR-005), not a generated input. Phase 3's "pin the
  generated-code shape" step is the checkpoint that re-confirms this split before
  Phase 8 bakes it into the emitter.
- **The overrun interceptor (Architecture §10/§20/§21).** Per-component deadline-overrun
  logging is a Yama-authored interceptor auto-attached to every component — internal, no
  public API. Its log sink is stdlib `log/slog` by default (Phase 2 pins this
  concretely; see Phase 2 for the exact checkable requirement).
- **A shared internal bridge package (`internal/bridge` or similar), required by
  Go visibility, named by no ADR.** `package yama`'s context-carrier key type is
  unexported (Phase 1), but the per-component wrapper that *sets* that identity
  lives in the runtime-support package (ADR-010) — a different package. The shared
  bits (the context-key type, and the `Config` the boundary `Option`s accumulate)
  go in `internal/bridge` under the module root, imported by both `package yama`
  and the runtime-support package. `package yama` re-exports `FromContext` as a
  thin wrapper over it, keeping `package yama`'s literal exported surface exactly
  ADR-007's. Two Go mechanisms are both load-bearing and must not be conflated:
  - **Import-path visibility** (`internal/`): only code rooted at
    `l7e.io/yama/v2/...` may import `internal/bridge`. `package yama` and the
    runtime-support package qualify; **generated application code — a separate
    module — does not**, and reaches this machinery only through the
    runtime-support package's exported API (ADR-010). Phase 9's external-module
    build proves it.
  - **Identifier export** (capitalization): the bridge symbols the runtime-support
    package calls (`bridge.WithComponent`, `bridge.Config`) must be **exported** —
    import-path permission does not reach unexported identifiers.
  - **`Lifecycle` is deliberately *not* a shared bit.** It is a public *interface*
    (`Starter` + `Stopper`, Phase 1); the runtime-support package satisfies it
    structurally with a private type, so nothing needs importing and the bridge
    carries only the context key and `Config`.

Plan-level implementation choices not covered by the docs (by design), each stated
in the phase that uses it: the generator lives in `internal/generator` behind a
thin `cmd/yama`; Google Wire is pinned as a `go.mod` tool and invoked via `go tool
wire`; v1 is deleted in Phase 0; the `internal/bridge` package above; and **the
panic recovery point** (Phase 2/3 — ADR-006 (Component Panics) fixes the policy;
the plan-level choice is where recovery happens — at the operation/level-runner
boundary rather than inside the interceptor chain — applied uniformly to graph and
boundary components).

---

## Phase 0 — Repo disposition, module & tooling baseline

**Goal.** Remove the v1 signal-watcher (green-field rewrite) and establish the
v2 package skeleton and tooling so later phases have a clean, compiling home.

**Files/modules likely touched.** Delete `yama.go`, `option.go`, and the v1 tests
(`yama_test.go`, `yama_unix_test.go`, `yama_windows_test.go`, `example_unix_test.go`);
touch `go.mod`, `go.sum`, new package dirs, `README.md`, possibly `docs/adr/` (a
short new ADR recording the v1→v2 disposition, at the next free number).

**Dependencies.** None.

**Risk.** LOW — mechanical, **conditional on v1 having no external consumers**. This
project is pre-1.0 and v1 has not been tagged as a release; if that changes before
Phase 0 executes, re-rate this phase (a breaking deletion for an already-adopted
package is not a LOW-risk mechanical change). History is preserved in git;
optionally tag the pre-removal commit.

**Definition of Done.**
- *Process:*
  - Before deleting, confirm nothing outside the v1 files references their exported
    names (`NewWatcher`, `WithClosers`, `WithTimeout`, `FnAsCloser`, etc.) — grep
    the tree so the deletion leaves no dangling references.
  - Read ADR-002 and ADR-008 before laying out the generator (`internal/generator`
    behind a thin `cmd/yama`), since the generator must import
    `github.com/google/wire` public types.
- *Output:*
  - Repo compiles (`go build ./...`) and `go vet ./...` is clean after the chosen
    disposition.
  - `go.mod` declares the `github.com/google/wire` dependency (public types only;
    **no** `internal/` imports — verifiable by grep).
  - The v2 public package is importable, carries a package doc comment, compiles,
    and exports no symbols yet (`go doc` shows only the package synopsis); no
    dangling references to removed v1 symbols anywhere in the tree.
  - `golangci-lint run` passes on the (possibly empty) new packages with the
    existing `.golangci.yml`.

**Regression note.** With v1 removed, confirm no other file, test, or example
references its exported names before marking done, and that `go build ./...` /
`go test ./...` are clean with the v1 files gone.

---

## Phase 1 — Runtime public API & context carrier

**Goal.** Define **and freeze the entire public surface** every other artifact
targets: **the `Lifecycle` type and its `Start`/`Stop` methods** (ADR-007's
"primary lifecycle abstraction," Architecture Appendix C), the three capability
interfaces, the three interceptor interfaces, `ErrStartFailed`, the
component-identity context carrier + `FromContext`, and the *signatures*
(bodies may be stubs) of **all** public helpers — `WithBeginComponents`/`WithEndComponents`,
`WithInterceptors`, `RunUntilSignal` (per ADR-007 + Architecture Appendix C). After this
phase the API-surface golden is **complete**; Phases
3/4 add behavior to these symbols without changing their signatures.

**Files/modules touched.** `lifecycle.go` (the `Lifecycle` interface +
`Start`/`Stop` signatures, the capability interfaces, `ErrStartFailed`),
`interceptor.go`, `context.go` (unexported key type + accessor), `options.go`
(boundary option types), `helpers.go` (helper signatures + stub bodies), and
`internal/bridge/` — the shared internal package from Orientation §0, holding the
context-key type and the boundary `Config` as exported identifiers on a
path-restricted import. `package yama`'s `context.go` becomes a thin wrapper over
it, letting the Phase 2 wrapper (a different in-module package) set the same
identity the Phase 1 accessor reads. Generated code (Phase 8/9) never imports
`internal/bridge`; it reaches `Lifecycle` only through the runtime-support
package's exported API (ADR-010).

**Dependencies.** Phase 0.

**Risk.** **HIGH — external-facing API.** This phase *is* the permanent public
surface: every symbol is a long-term compatibility commitment (ADR-007), and a
wrong signature forces a breaking change and invalidates every downstream golden.
Constructor/helper signatures follow ADR-007 + Architecture Appendix C; the ADR-010
runtime-support symbols live in a separate package and do not enter `package
yama`'s frozen surface here.

**Definition of Done.**
- *Process:*
  - Read ADR-003, ADR-005, ADR-006, ADR-007 **and Architecture §12** in full before
    writing signatures; the non-uniform interceptor shapes are a deliberate,
    explicitly-argued decision (Start returns `error`; Quiesce/Stop do not) and must
    not be "normalized."
  - Include `FromContext` in the public surface (ADR-007 §"Public Context
    Accessor"); the operation is conveyed by the interceptor method, so do **not**
    add an operation-identity context accessor.
  - Do **not** add any symbol outside the public surface enumerated in
    Architecture's Appendix C (Public API Reference) — the concrete target this
    phase builds — which reflects the ADR-007 minimal-API decision (no `Graph`,
    `Component`, `Plan`, `Level`, `Register`, `SetLogger`, config types). Enforce with a
    committed **API-surface golden test** (e.g. `go doc`-style exported-symbol
    snapshot) that fails on any unlisted export.
- *Output checks:*
  - **`Lifecycle` exists as an exported interface composed of `Starter` and
    `Stopper`** — exactly `Start(context.Context) error` and `Stop(context.Context)`
    (Architecture Appendix C, ADR-007 §"Public Lifecycle Type"); `Quiesce` is
    **not** on `Lifecycle`, asserted by the API-surface golden. The implementation
    is private and owned by the runtime-support package, so `package yama` needs no
    shared `Lifecycle` type and `internal/bridge` carries only the context key and
    `Config`. An unconstructed (nil) `Lifecycle` panics by language guarantee — no
    hand-written guard to assert.
  - **`internal/bridge` compiles, and the identity round-trips through it:** a test
    attaches a component with `bridge.WithComponent` (the exported setter the
    runtime-support package will call) and `yama.FromContext`, reading through
    `package yama`'s thin wrapper, recovers exactly that value. It lives in
    `context_test.go` (`package yama_test`, rooted at `l7e.io/yama/v2`, so it may
    import `internal/bridge`), exercising the public accessor from outside the
    package. It guards that the accessor keeps delegating to `bridge` rather than
    growing a key of its own.
  - `Starter.Start(context.Context) error`, `Quiescer.Quiesce(context.Context)`,
    `Stopper.Stop(context.Context)` exist with exactly these signatures.
  - `StartInterceptor.Start(ctx, next Starter) error`,
    `QuiesceInterceptor.Quiesce(ctx, next Quiescer)`,
    `StopInterceptor.Stop(ctx, next Stopper)` exist; Quiesce/Stop interceptors
    return nothing. The API-surface golden pins these shapes — normalizing
    Quiesce/Stop to return an error fails it with a reviewable diff. Do **not** add
    a witness-only "conformance table" to assert them; a witness that exists only to
    be asserted proves nothing. Compile-time interface guards belong where a real
    implementation exists for its own reasons (Phase 2: `var _ StartInterceptor =
    (*overrunInterceptor)(nil)`).
  - `ErrStartFailed` is a package-level `error`; `errors.Is(x, ErrStartFailed)`
    works.
  - **`FromContext[T any](ctx) (T, bool)` yields the component itself**, not a
    framework-owned descriptor. There is no `Component` type and no
    framework-derived name: a component with a printable identity implements
    `fmt.Stringer`, else `%T` gives its type. Settled; rationale in ADR-007
    §"Public Context Accessor".
  - **`T` is unconstrained, and must stay that way.** Go cannot express "implements
    at least one of `Starter`/`Quiescer`/`Stopper`" (a union may not contain
    method-bearing interfaces; embedding demands all three; a marker interface
    either constrains nothing or makes the capabilities unimplementable outside
    `package yama`). Do not add a `type Component = any` constraint — a constraint
    that constrains nothing.
  - `FromContext(ctx)` returns the component when previously attached and
    the zero `T` + `ok=false` when absent or when the component is not a `T`. The
    context key is an **unexported** type (collision-proof) — verified by a test
    that a caller's own `context.WithValue(ctx, "component", …)` does **not** leak
    into the accessor.
  - Accessor exposes the component **only** — no graph/plan/error leakage, and
    **no operation identity** (that is conveyed by which interceptor method runs,
    per Architecture §12 / ADR-007); `FromContext[T]` returns the bare `T`, so no
    descriptor exists that could carry anything else.
  - **`Option.Apply(*bridge.Config)` is exported on purpose; do not make it
    unexported.** The runtime-support package must apply the options generated code
    hands it, so the method is exported; the seal is the parameter type
    (`*bridge.Config` is unnameable outside this module by the `internal/` rule, so
    a forged `Option` fails to compile). An unexported `applyOption` would compile
    but leave only `package yama` able to apply options, silently breaking the
    runtime-support package. Guarded by `TestOptionsApplyFromAnotherPackage` (in
    `package yama_test`), which stops compiling if `Apply` is unexported.
  - **All public-helper signatures are declared and compile** (`WithBeginComponents`,
    `WithEndComponents`, `RunUntilSignal`) with stub bodies (e.g. `panic("unimplemented")`
    guarded so it never ships, or a documented no-op). The API-surface golden
    includes them, so it is **complete at the end of Phase 1** and does not grow in
    Phase 4.
- *Edge/failure cases:* `FromContext` on a context carrying no identity
  (`context.Background()`, or any context Yama has not wrapped) returns the absent
  result (zero `T`, `ok=false`). A **nil** context panics, deliberately: Go's rule
  is "do not pass a nil Context", so a nil one is a caller bug and `(zero, false)`
  would mask it — no defensive guard.

**Regression note.** These signatures are consumed by Phases 2–10. Any change here
after Phase 5 forces regeneration and re-verification of all golden files
(Phase 8/10). Freeze early. The API-surface golden frozen here is re-verified (not
extended) after Phases 4, 8, and 9. A change to `internal/bridge` (the bridge)
forces re-running both this phase's cross-package fixture check and Phase 2's
identity-attachment tests, since both depend on the same shared type.

---

## Phase 2 — Interceptor chains + universal wrapper (runtime core)

**Goal.** Implement the runtime machinery that generated code will call: build the
three separate operation chains (global, registration-ordered — no per-component
scoping, ADR-005), attach component identity to context before the chain runs, and
the **universal
per-component wrapper** that gives every component per-component attribution (identity in context)
and threads the caller's context — *with its deadline* — unchanged.

**Files/modules touched.** chain builders, the per-component wrapper, and identity
attachment — in the **runtime-support package** (ADR-010; the package path, e.g.
`l7e.io/yama/v2/rt`, is ADR-010's own illustrative example, not a binding name —
pick and record the real module-qualified path here), exported so generated
app-package code can call them, and documented as not part of the stable ADR-007
API. Identity attachment goes through the **`internal/bridge` introduced
in Phase 1** — the wrapper imports it directly and calls its exported setter to
attach the same unexported-typed context value `yama.FromContext` reads;
it does **not** invent a second, incompatible key. All exercised via hand-authored
"as-if-generated" fixtures in `_test.go`.

**Wrapper vs. level runner (component boundary).** Phase 2 and Phase 3 share
ownership of per-component execution, so the split is stated explicitly to avoid the two
being conflated: the **per-component wrapper** (this phase) is a single function/closure
around one component's operation — it attaches identity, threads context, and runs the
interceptor chain, and it is the unit Phase 8's generated per-component calls invoke
directly. The **level runner** (Phase 3) is a distinct, separately-testable helper
that *calls* the wrapper for each member of a level, adds `recover`, and applies
fail-fast/ordering policy across the members. Panic recovery lives in the level
runner, never inside the wrapper — the wrapper must let a panic propagate
unmodified out of the interceptor chain and back to whatever called it.

**Dependencies.** Phase 1.

**Risk.** HIGH — this is the shared execution substrate; a bug here is a bug in
*every* generated app. Concurrency-adjacent and behavior-modifying.

**Definition of Done.**
- *Process:*
  - Read ADR-005 §"Separate Lifecycle Chains", §"Ordering", §"Universal Wrapping",
    and Architecture §10, §20 before implementing.
  - **Overrun mechanism (Architecture §10/§20/§21):** overrun logging is a
    **Yama-authored interceptor, auto-attached to every component** (internal, no exported
    API); the wrapper only attaches component identity and threads the caller's
    context (deadline intact). **Log sink, pinned as a concrete, checkable
    requirement (plan-level default; not specified by the docs):** the interceptor
    logs via the stdlib `log/slog` default logger at `Warn` level, one line per
    overrun, including the component and the elapsed-vs-deadline delta; it
    identifies the component by `String()` when it implements `fmt.Stringer` and by
    `%T` otherwise (there is no framework-derived name — see Phase 1). It
    takes no constructor argument to redirect or silence the sink in this phase. A
    test asserts an overrun produces exactly one `slog` record matching that shape;
    the "can it be silenced" question is deferred, out of scope, and not claimed as
    done here. See the overrun check below.
  - **Panic policy — documented in ADR-006 (Component Panics); its recovery point
    is owned by Phase 3** (recovery at the operation/level-runner boundary, not
    inside the chain). Phase 2 only asserts the chain/wrapper do not *themselves*
    recover or swallow a panic — they let it reach the Phase 3 boundary. The policy Phase 3
    implements: Start panics recovered there and converted to `ErrStartFailed`;
    Quiesce/Stop panics recovered there and swallowed so the traversal completes. An
    interceptor that wants to *observe* a panic wraps `next` with its own `recover`.
  - Every component is wrapped whether or not an interceptor is attached (universal
    wrapping is not opt-in) — assert this explicitly.
- *Output checks:*
  - Chain execution order equals registration order: given `[Telemetry, Metrics,
    Logging]` the observed call order is Telemetry→Metrics→Logging→component
    (Architecture §10). A test asserts the exact order string.
  - Interceptors apply globally to all components that implement the matching
    operation-specific interface — there is no per-component scoping (ADR-005
    Non-Goals).
  - Only interceptors implementing the operation-specific interface join that
    operation's chain (a type implementing only `StartInterceptor` never runs in
    the Stop chain).
  - An interceptor can: (a) observe, (b) modify context seen by `next`
    (`FromContext` and a custom value both visible downstream),
    (c) **suppress** execution (return without calling `next`, and the component
    method is provably not invoked), (d) modify the outcome (turn a component
    error into `nil` for Start).
  - **Duration is measurable through the wrapper (PRD §6.8).** An interceptor that
    times `next` observes a duration that covers the whole wrapped operation
    (start-to-end visibility is preserved: the wrapper does not swallow or truncate
    the interval). Asserted with a fake whose operation sleeps a known minimum and
    an interceptor that records the measured elapsed time.
  - **Tracing metadata survives the wrapper (PRD §6.8).** PRD §6.8 lists tracing
    alongside logging/metrics/telemetry as metadata the framework must support; this
    is exercised directly, not only implied by the general context-modification
    check above: an interceptor attaches a synthetic span/trace value to context via
    `context.WithValue` before calling `next`, and a nested interceptor (or the
    component itself, via a test hook) reads it back unchanged — proving the wrapper
    neither strips nor replaces caller-attached context values it doesn't own.
  - The chain/wrapper do **not** recover panics themselves (verified: a panic from
    `next` reaches the caller of the chain, i.e. the Phase 3 boundary, unrecovered).
    Full panic-outcome behavior (Start→`ErrStartFailed`, Quiesce/Stop→swallowed) is
    asserted in Phase 3 where recovery lives.
  - **The built-in overrun interceptor reports per-component overrun (Architecture
    §20/§21).** With a
    context whose deadline fires before a component's operation returns, the
    Yama-authored auto-attached interceptor reports the overrun **exactly once**,
    attributed via `FromContext(ctx)`, to the chosen sink (assert against a
    test sink). It does **not** cancel or abandon the operation; the wrapper waits
    for `next` to return (asserted with a controllable slow fake). No exported
    overrun API is involved.
  - Chains are built **once** at construction and reused; the wrapper does not
    rebuild chains per call (asserted via a build-counter).
- *Edge/failure cases:* empty interceptor set → component still invoked exactly once;
  no `WithInterceptors` call at all → same, `Option`s default to empty.

**Regression note.** Phase 8 emits code that calls these runtime-support helpers
(ADR-010). If a helper signature changes after Phase 8, regenerate goldens. The
suppression/outcome-modification semantics interact with Phase 3's fail-fast:
re-verify Phase 3 tests whenever chain behavior changes. The dependency also runs
in reverse: if Phase 3's level-runner panic-recovery policy changes, re-run this
phase's "chain/wrapper do not recover panics themselves" check — a level-runner
change is exactly the kind of edit that tempts moving `recover` into the wrapper by
mistake, which this phase's test is what would catch.

---

## Phase 3 — Shared execution helpers + the behavioral contract (NOT a generic engine)

**Goal.** Build the **shared execution helpers** the generated orchestration relies
on — the per-component wrapper (Phase 2), a boundary runner, an `errgroup`-style
intra-level concurrency helper with fail-fast, per-component "started" tracking,
and the quiesce-then-teardown shutdown/cleanup plumbing — and establish the
**behavioral contract** (ordering invariants, fail-fast, boundaries) that Phase 8's
*generated* code must later satisfy. Ordering is carried by the order in which the
generated constructor declares its levels; the runtime-support package holds that
level list and drives each pass over it.

**Files/modules touched.** The boundary runner, intra-level concurrency helper,
started-flag tracking, and cleanup plumbing — in the **runtime-support package**
(ADR-010), exported for generated app-package code to call. Exercised via
hand-authored **stand-ins that replicate the exact generated code shape** pinned
below — not a general engine.

**Dependencies.** Phases 1–2.

**Risk.** **HIGH — data-integrity critical** *and* **architectural**. Data: wrong
shutdown ordering or a broken cleanup path corrupts real applications (a dependency
torn down while a dependent still uses it). Architectural: the standing temptation
here is to build a hand-written orchestration engine, which ADR-004 rejects — the
DoD process checks below guard against that.

**Definition of Done.**
- *Process:*
  - Read ADR-003 (all lifecycle semantics + Invariants 1–6), **ADR-004**, ADR-009
    (boundaries), Architecture §8, §15–§20, §22 before implementing.
  - **Pin the generated-code shape first (the ADR-010 revisit checkpoint; resolves
    the "designing around fixtures" concern).** Before writing any fixture, settle
    what the generated constructor will look like (the shape Phase 8 emits). The
    hand-authored test stand-ins in this phase MUST match that shape, so Phase 8
    can later emit code of the same shape and inherit this phase's behavioral suite.
    **Pin it as a checked-in artifact, not only prose**: `internal/generator/sandbox`
    is a hand-authored application package whose `lifecycle_gen.go` is exactly what
    Phase 8 must emit, and Phase 8's golden-emission tests diff their *shape*
    against it (call sequence, level membership — not byte-identical, since real
    graphs vary). This turns "Phase 8 conforms to Phase 3's shape" from an
    implicit expectation enforced only by shared behavioral tests into a checkable
    artifact: if Phase 8 drifts, the shape-diff fails independently of whether the
    behavioral suite happens to still pass. The exemplar is a **built** package, not
    `testdata` or a `.example` file, so the compiler catches it drifting from the
    runtime-support API it calls.
  - Run the full test suite under `-race` for every check below; a passing but
    racy implementation is not done.
  - Do not introduce any timeout/deadline of Yama's own — the only deadline is the
    caller's context (ADR-003 §"Stop Deadline", Architecture §20). This is an
    absence property over an open set of mechanisms, so it is not testable; hold
    it at review. What is testable is its consequence, checked below: a hung
    component stalls everything after it in the traversal.
  - **Boundary option ownership:** `WithBeginComponents`/`WithEndComponents` *signatures* are
    fixed in Phase 1; their *behavior* is implemented and tested here. Phase 4 does
    **not** revisit them (removes the earlier "finalize in Phase 4" overlap).
- *Output checks (ordering invariants):*
  - **Invariant 1:** a dependency's `Start` completes before any dependent's
    `Start` begins (fixture with A→B→C asserts start order and that same-level
    independent components overlap in time).
  - **Invariant 2:** a dependent's `Stop` completes before its dependency's `Stop`
    begins (reverse order asserted).
  - **Invariant 3:** *every* component's `Quiesce` completes before *any*
    component's teardown `Stop` begins (a global "phase" observer proves no
    interleave).
  - Quiesce runs in **reverse dependency order** (dependents quiesce first), same
    direction as Stop — and ordering **holds transitively through non-`Quiescer`
    components** (A→(non-quiescer)B→C: A quiesces before C).
  - Independent branches start/quiesce/stop **concurrently** (timing or
    barrier-based proof, not just "no error").
  - Components implementing only some capabilities: a `Starter`-only component never gets
    `Quiesce`/`Stop`; a `Stopper`-only component never gets `Start`.
- *Output checks (fail-fast + cleanup):*
  - Startup failure in the active level: no later level is started (asserted:
    level-N+1 components' `Start` never called), in-flight same-level ops are
    awaited to settle **uninterrupted** (asserted: a sibling's failure does not
    cancel them), and **only the successfully-started** components are
    quiesced+stopped (scoping proven with a component that failed vs. a sibling
    that started).
  - The cleanup path is **the same code** as normal `Stop` (Invariant 4) — proven
    by a shared observer seeing identical ordering, not a parallel implementation.
  - `Start` returns exactly `ErrStartFailed` (via `errors.Is`) and never the
    component's error; `Stop` returns nothing.
  - **The runtime-support package's `Lifecycle` implementation is never a silent
    no-op.** A nil interface value panics by language guarantee (Phase 1); this
    phase must not reintroduce the hazard at the other end — an implementation that
    returns `nil` from `Start` while orchestrating nothing. `Start` reports success
    only when components actually started.
  - **`Stop` is idempotent** (the framework guarantee that makes a once-only helper
    unnecessary — see Phase 4): calling `Stop` more than once, or concurrently, runs
    the quiesce+teardown passes **once**; later/overlapping calls observe the same
    completion and re-trigger nothing. Race-tested. (Applies too when `Start`
    already ran startup-failure cleanup and the app then calls `Stop`.)
  - **Start deadline exceeded → `ErrStartFailed` (PRD §6.9 / ADR-006 §"Timeout
    Errors").** A component that *honors* the caller's context and returns an
    error when its deadline fires is handled as an ordinary start failure: `Start`
    returns `ErrStartFailed` (not a distinct timeout error), the startup context is
    canceled, later levels are not started, and startup-failure cleanup (quiesce +
    teardown) runs over the successfully-started components.
    **The fixture must honor `ctx` — e.g. `select { case <-ctx.Done(): return
    ctx.Err() }` — not merely sleep past the deadline.** Yama never converts a
    deadline into an error and never preempts a component (Architecture §20: the
    deadline is observational; the overrun interceptor logs it and the framework
    keeps waiting). A fixture that ignores `ctx` and blocks would therefore hang
    this test forever rather than fail it. What is asserted here is that a
    component's *own* deadline error is treated like any other start failure —
    Yama adds no timeout handling of its own.
  - **Caller context already canceled at `Start`:** `Start` observes the caller's
    context and, finding it already canceled or past its deadline, returns
    `ErrStartFailed` before any level runs — no component is started and nothing is
    torn down. Because the start ran nothing, the lifecycle is left unchanged and
    stays startable under a live context; it is **not** driven into the spent
    terminal state a real start failure produces (ADR-006 §"Canceled Start
    Context"). Asserted, including the retry under a live context.
  - A hung component stalls the traversal (does **not** return early) — a test
    with a bounded wait confirms the traversal blocks on it and later components have
    not run; document that only SIGKILL bounds this (do not add a framework
    escape).
  - **Panic policy — one exact rule, recovery at the operation/level-runner
    boundary:**
    - A **Start** panic (component or interceptor) is **recovered** at the level
      runner and converted to a failed start: it fails the level, cancels the
      startup context, triggers startup-failure cleanup, and `Start` returns
      `ErrStartFailed` (via `errors.Is`). It does **not** propagate as a panic.
    - A **Quiesce/Stop** panic is **recovered** at the level runner and
      **swallowed** so the traversal runs to completion; nothing is returned.
    - Yama surfaces the panic no further; an interceptor that wants to observe it
      wraps `next` with its own `recover`.
    - Asserted for: a Start-component panic, a Start-interceptor panic (both →
      `ErrStartFailed` + cleanup ran), and a Stop-component panic (→ traversal
      completes, later dependencies still torn down).
- *Output checks (boundaries, ADR-009):*
  - Boundaries are dependency extremes: a begin component starts before all graph components
    and quiesces/stops **after** all of them (a base every graph component depends on); an
    end component starts after all graph components and quiesces/stops **before** all of them
    (a top that depends on the whole graph). Shutdown is the exact reverse of startup —
    `end → graph → begin` — for the Quiesce and Stop passes, and also in startup-failure
    cleanup (same passes, no separate path). A non-`Starter` boundary component takes part in
    shutdown only if its boundary was reached (begin always is; end only if startup got past
    the graph).
  - A boundary component joins a pass only if it implements that pass's interface.
  - Boundary failure handling is **identical to a graph component's, not a separate
    policy** (ADR-009): a begin/end `Starter`'s error or panic fails startup
    (`ErrStartFailed` + cleanup ran) exactly as a graph `Starter`'s would; a begin/end
    `Quiescer`/`Stopper`'s error or panic is recovered and swallowed exactly as a graph
    component's would — asserted for both error and panic, and for both begin and end
    placement.
  - Boundary sets are unordered/concurrent (no ordering assertion is made or
    required; a test must not depend on intra-set order).
- *Edge/failure cases:* zero components; a single component with all/none
  capabilities; a dependency-only value with a cleanup and no capability of its
  own (occupies a level and receives a `Stop` call that runs only the cleanup, no
  `Start`/`Quiesce` callback); a `Stop` context that is already dead on arrival
  stops neither pass, over a graph deep enough that a between-level bail would
  show (the harder form of "canceled mid-shutdown": the context is spent before
  the first pass begins, so an implementation that consults it at entry or
  between levels fails, and `Stop` returns nothing either way); no goroutine
  leaks out of the intra-level fan-out — asserted with `goleak` per spec, scoped
  by `IgnoreCurrent` so a failure names the spec that leaked rather than the one
  that observed it, over each path that fans out (a level starting its members at
  once, a member failing while its siblings are still running, and concurrent
  `Stop`), with a suite-wide `VerifyTestMain` backstop covering the paths not
  enumerated. (The canceled-at-`Start` and start-deadline cases are covered as
  first-class output checks above.)

**Regression note.** Phase 7 folds cleanup functions into the teardown pass — re-run
**all** Phase 3 ordering tests after Phase 7 to confirm they occupy the correct DAG
position and don't perturb ordering. Phase 8's emitted code
must declare levels that satisfy these same invariants — Phase 3's tests
become the behavioral contract Phase 10 re-checks on generated output.

---

## Phase 4 — Public helper: `RunUntilSignal`

**Goal.** Implement the **body** of `RunUntilSignal`; its signature is frozen in
Phase 1, so this phase changes behavior, not signature.
(`WithBeginComponents`/`WithEndComponents` are fixed in Phase 1 and implemented in
Phase 3 — not revisited here.) `RunInBackground` and `EnsureExactlyOnce` are this
plan's shorthand for helpers it does **not** build — no canonical doc names them.
ADR-007 §"Public Helpers" establishes why they are unnecessary: a blocking `Start`
is the component's own responsibility to background, and `Stop`'s idempotency
(Phase 3) removes any need for a once-only helper.

**Files/modules touched.** `helpers.go`, `signal.go`, `helpers_test.go`
(plus unix/windows-tagged signal tests mirroring the existing test layout).

**Dependencies.** Phases 1–3.

**Risk.** LOW-MEDIUM — `RunUntilSignal` touches OS signals and must behave across
unix/windows (CI matrix includes both); otherwise thin.

**Definition of Done.**
- *Process:* read ADR-007 §"Public Helpers" and Architecture §15/§16 for the
  intended role and the "component backgrounds its own blocking `Start`" /
  "idempotent `Stop`" guidance that replaced the dropped helpers.
- *Output checks:*
  - `RunUntilSignal(lc, signals...)`: starts `lc`, blocks until one of the signals
    (default `SIGINT`/`SIGTERM` when none passed — the interrupt/termination default
    documented in PRD §6.12), then calls `Stop` and returns; returns
    `ErrStartFailed` if `Start` failed, without waiting for a signal. Signal
    delivery is tested on unix; the windows-tagged variant compiles and uses the
    platform-appropriate signal set.
- *Edge/failure cases:* a second signal during shutdown does not re-enter `Stop`
  (which is idempotent anyway — see Phase 3); `Start` failure short-circuits before
  any signal wait.

**Regression note.** `RunUntilSignal` shapes the generated `main`/startup pattern.
A **behavioral** change (not just a signature change) can break generated lifecycle
behavior — after any change here, re-run the Phase 10 end-to-end integration test,
not only the golden diff.

---

## Phase 5 — Generator: run `wire gen` + parse the injector AST

**Goal.** Build the generation front-end: invoke `wire gen`, then walk the
AST of each injector function in the resulting `wire_gen.go`, extracting the
ordered creation events, the dependency edges (argument consumption), the
returned root, and detected legacy cleanup functions — **without** yet
computing lifecycle levels.

**Files/modules touched.** `internal/generator/parse*.go`, `cmd/yama` (thin
entry), test fixtures under `internal/generator/testdata/…` containing
real Wire injector inputs.

**Dependencies.** Phase 0 (module + wire dep). Independent of Phases 1–4.

**Risk.** **HIGH** — the whole design couples to the *shape* of Wire's generated
output (ADR-008 explicitly accepts this, guarded by the Phase 10 drift check).
Unsupported injector shapes must fail loudly at build time, not silently mis-order.

**Definition of Done.**
- *Process:*
  - Read ADR-002, ADR-008, and Architecture §4–§6 in full before implementing;
    the "what counts as a creation event" rules are enumerated there and must be
    followed exactly.
  - **Must not** import or reference any `github.com/google/wire/internal/…`
    package (grep-verified); only public Wire types and the generated source text.
  - Use real fixtures run through actual `wire gen` (Architecture §24), not
    hand-faked `wire_gen.go`, so the parser is tested against genuine output.
- *Output checks:*
  - Statement order of the injector body is preserved as a valid topological order
    (top-to-bottom = dependencies-before-dependents).
  - **Every** new-variable creation form is captured as a component: call-expression
    providers, `wire.Value`/`InterfaceValue` assignments, and
    `wire.Struct`/`FieldsOf` struct literals (one fixture per form; each asserts
    the component is present).
  - The returned value (e.g. `App`) is recorded as the injector's root **and**
    retained as an ordinary component with its own dependency edges.
  - Each injector function is parsed as an **independent** graph; a file with two
    injectors never merges them (asserted).
  - Dependency edges are derived from the arguments each creation event consumes
    (A depends on B iff A's creation uses B's value).
  - Wire cleanup functions (`func()` returns / the `cleanup` var pattern)
    are detected and associated with the value they clean up (position recorded
    for Phase 7).
- *Edge/failure cases:*
  - An unsupported / unrecognized injector shape produces a **build-time error**
    with a clear message (not a panic, not silent omission) — the message names the
    injector function and the offending statement/position, so a user can locate the
    problem without reading the generator's source. Asserted against **at least
    three** distinct malformed-shape fixtures, not one generic case: (a) a creation
    form the parser does not recognize (e.g. a raw composite literal that is not
    `wire.Struct`/`FieldsOf`), (b) a statement shape that produces no clear
    dependency edges (e.g. a value used only through an unexported field the parser
    cannot see into), (c) two injector functions in the same file whose root types
    collide in a way naming resolution cannot disambiguate deterministically. Each
    fixture asserts both the error and the (non-)panic behavior, and that the error
    message content differs meaningfully per case (proving the message is
    shape-specific, not one canned string).
  - `wire gen` failing (bad Wire input) surfaces as a generation error that
    names the underlying failure, distinct from a toolchain error (`wire` is
    pinned in `go.mod` and invoked via `go tool wire`).
  - Minimal injector, whose only creation event is the value it returns, parses
    cleanly and yields that single component.

**Regression note.** This phase's output shape (component/edge model) is consumed by
Phases 6–8. Freeze the internal component representation before Phase 6. Wire version
bumps can change output shape — Phase 10's drift check is the guard; note the
tested Wire version.

---

## Phase 6 — Generator: lifecycle capability detection + level computation

**Goal.** From the parsed graph, determine each component's lifecycle capabilities and
compute **one dependency-ordered level list per injector**, with **transitive ordering
preserved through dependency-only / non-capable components**. Startup runs that list
forward; quiesce and stop run it back. The runtime built in Phases 1–4 takes a single
`[]exec.Level` and walks it in both directions, so the reverse orderings are a
consequence of the one list rather than separately computed structures.

**Files/modules touched.** `internal/generator/analyze*.go`,
`internal/generator/levels*.go`, fixtures + expected-level assertions.

**Dependencies.** Phases 1 and 5. (Phase 5 supplies the parsed component/edge model;
Phase 1 supplies the capability interface definitions that capability detection
resolves against — "implements `Starter`" is decided by static type analysis of the
generated package, not runtime reflection.)

**Risk.** **HIGH — the correctness crux of the entire project.** This is where the
ordering guarantees the runtime relies on are actually *decided*. A wrong level
assignment yields silently-wrong startup/shutdown ordering in every consuming app.

**Definition of Done.**
- *Process:*
  - Read ADR-003 §"Startup/Quiesce/Stop Semantics" and Architecture §7 before
    implementing; the transitive-through-non-capable-component rule is stated in both
    and is the most error-prone requirement.
  - Capability detection is done by **static type analysis** of the generated
    package (does the concrete type satisfy `Starter`/`Quiescer`/`Stopper`?), with
    **no reflection** and no runtime discovery (ADR-001, PRD §4).
- *Output checks:*
  - Components with no lifecycle ordering edge between them share a level (may run
    concurrently); a dependent is in a strictly later level than its dependency (the
    diamond `DB → {Router, Worker}` yields the list `[DB]`, `[Router, Worker]` —
    matching ADR-003's worked example). Walked back for quiesce and stop, that same
    list is `[Router, Worker]`, `[DB]`.
  - **Transitive ordering through non-capable components:** for capable A → non-capable
    B → capable C, A and C remain correctly ordered relative to each other in the
    single list, and therefore in all three passes (dedicated fixture; this is the
    headline edge case).
  - Every lifecycle-capable component occupies a level, and so does every
    dependency-only component whose provider returned a cleanup — its cleanup runs
    at that position even though it receives no callback. A dependency-only
    component with neither trait occupies no level; it still transmits ordering. A
    component receives a callback only for the capabilities it implements, so a
    non-`Quiescer` transmits quiesce ordering without receiving a quiesce callback;
    no separate per-pass level set expresses that.
  - Level computation is **deterministic**: identical input yields byte-identical
    level assignment across runs (stable component iteration order — asserted by
    repeated runs), so goldens in Phase 8 are stable.
- *Edge/failure cases:* a component implementing all three vs. exactly one capability;
  a graph with no capable components (empty level list, no error); a wide fan-out
  (many independent components in one level); a deep chain (many single-component levels);
  a component that is both depended-upon and a dependent (interior of a chain);
  a **capable root**.

**Regression note.** Folding cleanup functions (Phase 7) adds teardown work at the
cleaned-up value's position in the level list. Re-run all Phase 6
level tests after Phase 7 to confirm cleanups land in the right level and don't
shift other components. This is the highest-value regression
checkpoint in the plan.

---

## Phase 7 — Generator: cleanup folding

**Goal.** Fold **every** Wire cleanup function into its value's position in the
level list (ordering only, type graph unmodified): alongside the value's own
`Stop` when the value is lifecycle-capable, or as the value's entire teardown when
it is not. The change is small — folding + positioning — but its stakes are not: a mispositioned
or dropped cleanup is the same "dependency torn down while a dependent still uses
it" failure Phase 3 rates HIGH, in the path much real resource teardown (DB pools,
file handles, connections) runs through. Kept a separate phase to hold the cleanup
semantics and the both-a-`Stopper`-and-a-cleanup case in one place. Background:
ADR-008 §"Cleanup functions".

**Files/modules touched.** `internal/generator/levels.go` (the member-kind
classification, alongside `Member`/`Capabilities`) and its tests, fixtures
covering plain-cleanup and both-a-`Stopper`-and-a-cleanup values, and the Phase 3
ordering suite (fixtures proving folded-cleanup teardown position). The folding
mechanism itself — `WithCleanup`/`WithCleanableComponent` — is `rt`-owned
(ADR-010, built in Phase 4); this phase decides which one applies to a given
value, it does not build the mechanism.

**Dependencies.** Phases 5–6.

**Risk.** **MEDIUM-HIGH** — a wrong teardown-form classification, or a folded
cleanup misplaced within its value's level, reintroduces the ordering bug the
project exists to prevent, in what ADR-008 calls the *primary* teardown
mechanism. Same data-integrity class as Phases 3/6/8, scoped down only because the
change is narrow and mechanical (classifying an existing component's teardown
form, not level computation or constructor semantics). The DoD below is held to
that standard.

**Definition of Done.**
- *Process:*
  - Read ADR-008 §"Cleanup functions" and Architecture §4 step 6, §17.
- *Output checks:*
  - Every detected cleanup function is folded into the teardown of the value it
    cleans up, at the **same DAG position** as that value, so Phase 6's shutdown
    levels place it correctly (verified against Phase 3's ordering invariants on
    the generated result). The folding helper's location/visibility follows
    ADR-010 — it must be constructible from generated app-package code.
  - The injected **type graph is unmodified** — folding affects ordering only
    (assert the app's own types are untouched).
  - Cleanups do **not** pass through interceptor chains; a cleanup is a plain
    `func()` with no context and no component identity. Asserted: an interceptor
    registered over a graph with cleanups observes the components' own operations
    and not the cleanups.
  - **Both-a-`Stopper`-and-a-cleanup value:** a provider that returns a cleanup func
    *and* whose value implements `Stopper` yields **one** teardown participant whose
    `Stop` runs the cleanup and then the value's own `Stop`, in that order and in the
    same teardown level. Asserted; **not** an error and **not** two components. Emitted
    via `WithCleanableComponent`.
  - **A cleanup with no other capability** emits via `WithCleanup` rather than
    `WithCleanableComponent`: the value contributes no `Starter`/`Quiescer`/`Stopper`
    behavior of its own, so nothing is gained by wrapping a component that implements
    nothing. It still occupies the value's DAG position.
- *Edge/failure cases:* a value with a cleanup func but no other capability (the
  cleanup is its only teardown role); multiple cleanup funcs in one injector (each
  folded at its own position); a `Stopper` value with no cleanup func (plain
  `Stopper`); a cleanup returned by the **root's** provider (folds at the root's
  position like any other).

**Regression note.** See Phase 6 note — this phase is the reason Phase 6's tests
must be re-run. Also re-run Phase 3 ordering tests against a fixture whose ordering
depends on a folded cleanup's position, and against a both-a-`Stopper`-and-a-cleanup
fixture (two teardown components, same level).

---

## Phase 8 — Generator: code emission, naming, formatting

**Goal.** This phase discovers the package's lifecycle stubs. It derives one
Wire injector from each stub. It emits `lifecycle_gen.go`: a provenance
header, then one constructor per stub. Each constructor returns
`(*App, Lifecycle, error)`. Each constructor re-emits its derived injector's
body, then declares each level and its members. The emitted file is
`gofmt`-clean and deterministic. Per-component wrapping and interceptor-chain
construction happen inside the runtime-support package, not in emitted code.
The public `WithInterceptors` option feeds the interceptor chain. The
emitted code supplies no interceptor input of its own.

**Stub discovery and injector derivation (ADR-011).** A hand-authored stub
behind `//go:build yamainject` supplies the constructor's name, signature,
and provider set. No application injector supplies any of these. This phase
adds a third package-load mode to `internal/generator/wire.go`, alongside the
existing `discoveryBuildFlags` and `parseBuildFlags` modes. The new mode
loads the package under the `yamainject` tag. It collects each function whose
body is `panic(wire.Build(…))`. It derives one Wire injector per stub. Each
derived injector takes its name from the stub's name, in the `yama`-prefixed
namespace. Each derived injector's parameters are the stub's parameters,
minus the trailing `opts ...yama.Option`. Each derived injector's results
replace `yama.Lifecycle` with `func()`. Each derived injector's body is the
stub's body, copied unchanged. The generator writes the derived injectors to
a transient `wireinject`-tagged file. It removes that file together with
`wire_gen.go`. `internal/generator/transient.go` already generalizes to a
second transient filename, so this reuses that mechanism. Wire diagnostics
land on the derived file, not the stub. Before the generator reports a
diagnostic, it maps the diagnostic's position back to the stub.

**Files/modules touched.** `internal/generator/emit*.go`; new file(s) for
stub discovery and derivation; `internal/generator/transient.go` (adds the
second transient name); the fixture corpus at
`…/testdata/<case>/want/lifecycle_gen.go`.

**Emission mechanism (this phase's choice).** The emitter assembles the file
as text and runs `go/format` over the result. The construction body is not
re-rendered from a template: each statement is printed from the parsed
`wire_gen.go` AST with `go/printer`, which is what keeps the re-emission
faithful to what Wire wrote. A template engine and a `jennifer`-style builder
were both rejected, since neither reproduces an existing AST and both would
mean re-deriving statements Wire already emitted. Google Wire's own generator
assembles source text and formats it the same way.

**Fixture corpus, not the sandbox.** Goldens follow Google Wire's own test
layout. Each case is a self-contained fixture package with a `want/`
directory holding the expected output. The golden test materializes the
actual output and compares it against `want/`. An `-update` flag records a
new expected output. `internal/generator/sandbox` is the hand-authored
exemplar. It is not a generation target. `internal/generator/testdata/sandbox`
is the frozen parse/analysis fixture. It is not a generation target either.

**Dependencies.** Phases 1–7. This phase needs both the runtime contracts
and the analysis.

**Risk.** **HIGH.** Phase 6 decides ordering. This phase owns constructor
semantics, and those semantics are data-integrity sensitive: the constructor
captures and discards Wire's raw cleanup function, so teardown runs only
through `Stop`. A mistake here double-tears-down resources, or strands a
cleanup so it never runs. Naming stability and deterministic collision
resolution also live in this phase.

**Definition of Done.**
- *Process:*
  - Read ADR-004, ADR-007, ADR-011, and Architecture §8, §13, §14, and
    Appendix C before emitting. Keep every generated implementation symbol
    in the `yama`-prefixed private namespace. Keep the public surface
    exactly matching Architecture's Appendix C (Public API Reference).
- *Output checks:*
  - The emitted file starts with `// Code generated by Yama. DO NOT EDIT.`.
    The file is `gofmt`/`goimports`-clean:
    re-formatting it is a no-op.
  - The generated package compiles. Wired to the Phase 1–4 runtime, it
    passes the Phase 3 behavioral contract: the ordering invariants,
    fail-fast, cleanup, and boundaries checks. The same behavioral suite
    exercises the generated code directly. A golden diff alone does not
    satisfy this check.
  - Generated code compiles against only the surfaces frozen earlier. This
    phase adds no late-breaking exports. Re-running the Phase 1
    API-surface golden after emission shows no new exported symbols in
    `package yama`. Generated code imports only two things: the frozen
    `package yama` surface, and the documented runtime-support package
    (ADR-010, e.g. `rt`). The app-facing generated symbols are the
    constructors the stubs declare. These constructors live in the
    application package, not in `package yama`. Assert that they add no new
    export to the framework surface. Interceptors reach the generated code
    through the public `WithInterceptors` option, supplied at the call
    site. So no generated interceptor input exists to check here.
  - The constructor returns `(*App, Lifecycle, error)`. On construction
    failure it returns `nil, nil, err`. On success it captures and discards
    Wire's raw cleanup function, so teardown runs only through
    `Lifecycle.Stop`. Assert that Wire's cleanup is never invoked directly,
    and that `Stop` tears down exactly once.
  - Interceptors supplied via `WithInterceptors` apply globally, to every
    component implementing the matching operation-specific interface.
    ADR-005's Non-Goals rule out per-component scoping, string keys, and a
    map.
  - `FromContext` yields the component instance the generated code is
    invoking. Assert this for a graph with two same-typed components: each
    component's chain sees its own instance, not the other's. There is no
    generated component name to check. Naming is the component's own
    business, via `fmt.Stringer` (Phase 1). Generated identifier naming,
    for the component locals, is a separate matter. That naming is still
    deterministic and collision-suffixed. The check above already covers
    it. It is internal to the generated code, and an application never
    observes it at runtime.
  - Golden test: representative graphs (a chain, a diamond, a wide
    fan-out, mixed capabilities, cleanup functions) each produce a stable
    golden `lifecycle_gen.go`. Re-emitting yields byte-identical output.
- *Edge/failure cases:* an empty graph, which is a valid minimal file whose
  constructor still returns a usable `Lifecycle`; a single component; a
  component whose name would collide with a Yama-reserved identifier,
  resolved by deterministic escaping.

**Regression note.** Golden files are coupled to *both* the runtime API
(Phases 1–4) and analysis (Phases 5–7). Any change upstream regenerates
goldens. A golden diff is expected. Review it deliberately. Do not accept
it blindly. Add a comment in the golden test explaining that a diff means
"confirm the change was intended."

---

## Phase 9 — Generation driver: one command, `go:generate`

**Goal.** This phase wires the pieces into a single user-facing generation
step: one `go:generate` directive, and one command. That command runs `wire
gen`, parses the result, and emits `lifecycle_gen.go` in the target package.
`lifecycle_gen.go` is the only committed output. `wire_gen.go` is a transient
intermediate. Yama removes it when done (see the non-destructive-generation
check below).

**Files/modules touched.** `cmd/yama/main.go`, an example app under
`examples/…` with a `//go:generate` directive, and the README usage section.
The example app is its own separate Go module, with its own `go.mod`,
requiring `l7e.io/yama/v2` as a normal external dependency. This makes
"generated code living outside the Yama module" a fact of the build. It is
the fixture the Orientation §0 external-module proof runs against. The
example's `go.mod` must resolve against the checked-out tree, not a
published or cached version. So the example's `go.mod` carries
`replace l7e.io/yama/v2 => ../..`, or the repo root carries a `go.work` file
with `use . ./examples/…`. At least one of the two must be present.
Otherwise CI could silently build a stale proxy version while the PR's real
changes go untested.

**Dependencies.** Phases 5–8.

**Risk.** MEDIUM. This is user-facing CLI and build ergonomics. It must fail
cleanly when the target package is malformed.

**Definition of Done.**
- *Process:* Read Architecture §4, §14 and ADR-008 §"Generation and drift".
- *Output checks:*
  - A single command, run in a fixture app, produces the committed
    `lifecycle_gen.go`. Running it twice is idempotent: the second run
    yields no diff.
  - Transient, non-destructive `wire_gen.go` handling (asserted). Google
    Wire writes `wire_gen.go` into the package directory, and cannot
    redirect it elsewhere. So the driver generates `wire_gen.go` there,
    parses it, and removes it after emitting `lifecycle_gen.go`. The
    driver removes only a `wire_gen.go` it created itself. If a
    `wire_gen.go` already exists, the driver first moves it aside, to a
    leading-dot name such as `.yama.wire_gen.go` (a name the Go toolchain
    ignores as a source file). The driver restores that file afterward,
    byte-for-byte. This way, generation never overwrites or deletes a file
    Yama does not own. Cleanup (the removal and the restore) runs even
    when the Yama step fails. So no `wire_gen.go` and no stray backup is
    left behind. Tests cover three cases: `wire_gen.go` pre-existing,
    `wire_gen.go` absent, and the failure path.
  - The `//go:generate` directive invokes Yama, which runs `go tool wire`
    internally. This directive is documented, and it works via `go
    generate ./…` on the example app.
  - Generation writes `lifecycle_gen.go` with one plain write. It is not
    made crash-safe. A write that fails part way can leave the file
    truncated, and another run restores it: generation reads only files
    that are still on disk, and it produces the same content every time.
    A staged write, renamed onto the file, was built and then removed. It
    bought recovery from a failure the next run already recovers from, and
    it cost the code that has to preserve the file's own permission,
    because a rename replaces the file rather than writing through it.
  - External-module compile proof (the Orientation §0 `internal/bridge`
    boundary). With the example app in its own `go.mod`, `go build ./...`
    and `go vet ./...` succeed for it as a standalone module. This check
    must also prove the build resolves the local checkout, not a stale
    published version. A stale version would "pass" while proving
    nothing. So assert that `go list -m l7e.io/yama/v2` (or `go env
    GOWORK`) reflects the local directory. Add a change-detection check
    too: a deliberately broken signature in local `package yama` or
    runtime-support code must break the example app's build. That break
    is the real proof. It shows that generated code, compiled from an
    external module, reaches `Lifecycle` only through the exported APIs.
    An accidental `internal/bridge` reference would instead fail here with
    Go's own `internal/` error, not a check Yama wrote.
- *Edge/failure cases:* the `wire` tool is unresolvable, e.g. not pinned in
  `go.mod` → a clear toolchain error, distinct from a Wire input error. The
  target package has no injector → a silent no-op, exit 0, matching `wire
  gen`'s own behavior (measured: Wire skips such a package whether named
  explicitly or matched by a wildcard, and exits 0 even when no named
  package holds an injector). A package pattern resolves to nothing → a
  clear error, since `wire gen` also fails there. The committed
  `lifecycle_gen.go` is read-only → a clear error, and the file keeps its
  content. A read-only output directory does not stop the write, because
  the write opens a file that is already there.

**Regression note.** The example app becomes a living integration fixture
reused by Phase 10. Keep it minimal, but ensure it covers at least 2
capabilities, one dependency-only component, and one cleanup function.

---

## Phase 10 — CI drift check, integration & golden coverage, docs

**Goal.** This phase adds three things. First, the CI drift check: it
regenerates `lifecycle_gen.go`, diffs it against the committed copy, and
fails on any divergence. Second, an end-to-end integration test that builds
and runs the example app's lifecycle. Third, it finalizes the docs and
README.

**Files/modules touched.** `.github/workflows/ci.yml`, `Makefile`/scripts,
`examples/…` integration test, `README.md`, possibly `docs/`.

**Dependencies.** Phases 1–9.

**Risk.** LOW-MEDIUM. This is internal tooling and CI. The main hazard is a
flaky or environment-sensitive drift check: for example, a Wire version
mismatch, a gofmt version mismatch, or OS line-ending differences.

**Definition of Done.**
- *Process:* Read Architecture §24 (testing strategy) and ADR-008
  §"Generation and drift". Pin the Wire version used in CI, and record it.
- *Output checks:*
  - CI regenerates `lifecycle_gen.go` for the example app. CI fails if the
    regenerated file differs from the committed copy.
  - A durable, committed self-test verifies the drift mechanism itself.
    This must be a script or `go test`, not a one-off manual
    tamper-and-revert. The test writes a known perturbation to a copy of a
    generated file. It runs the drift comparison. It asserts that the
    comparison reports a failure. Then it cleans up. This proves the
    guard actually catches drift on every CI run, not just once during
    review.
  - The drift check is stable across the CI matrix where it runs. Pin the
    OS, Go version, and Wire version it runs under. Run the drift job on a
    single well-defined platform, to avoid gofmt and line-ending noise. Do
    not run it across all three OSes unless it is proven stable there.
  - End-to-end test: the example app starts, in dependency order. It then
    receives a signal, or an explicit `Stop` call. It quiesces before
    teardown, and shuts down in reverse order. An interceptor-based
    observer asserts all of this, under `-race`.
  - Public error contract: an integration-level test confirms three
    things. `Start` returns only `ErrStartFailed` to callers. `Stop`
    returns nothing. Component errors are visible only through
    interceptors.
  - The README documents four things: the `go:generate` line; the `(app,
    lifecycle, err)` pattern; `RunUntilSignal` as the typical `main`; and
    the boundary-component options.
  - Large-graph generation is exercised (PRD §9, "Generated Code
    Complexity"). The example app used elsewhere in this phase is
    deliberately minimal. It does not exercise this named project risk on
    its own. So this phase adds one more fixture, separate from the
    example app and the drift check. That fixture is a wide, deep
    synthetic graph, on the order of 100 or more components. It runs
    through the full pipeline: `wire generate`, then Yama generation.
    Three things are asserted against it: (a) generation completes in
    bounded time: a generous, CI-stable ceiling, not a tight benchmark;
    (b) the emitted file still compiles and is `gofmt`-clean; (c)
    generated names stay unique and collision-free at that scale.

    This check needs a concrete, checkable placement, not just "some
    cadence." It runs as a named CI job, for example `large-graph` in
    `.github/workflows/ci.yml`. That job triggers on a weekly schedule (a
    `schedule:` cron trigger), and also manually via `workflow_dispatch`.
    It runs separately from the per-commit job, so it never blocks normal
    merges. This check's Definition of Done is satisfied once that job
    definition exists in the committed workflow file, and has been
    observed to pass at least once. An undocumented expectation that
    someone runs it "sometime" does not satisfy it.
- *Edge/failure cases:* the drift check's behavior when the `wire` version
  differs (documented; the version is pinned), and the coverage job (the
  existing Coveralls step), which still runs against the new packages.

**Regression note.** This phase locks the whole system. Any later change
to the runtime API, the analysis, or the emission logic will trip the
drift check. That is the intended behavior. Document the "regenerate, then
review the diff" workflow, so a legitimate change isn't mistaken for
accidental drift, and accidental drift isn't mistaken for a legitimate
change.

---

## Appendix: risk summary & regression map

| Phase | Area | Risk | Why |
|------|------|------|-----|
| 0 | Repo/module disposition | LOW | mechanical; removes v1 (green-field rewrite) |
| 1 | Public API surface | **HIGH** | external-facing API, a permanent compatibility commitment; freezes `Lifecycle`, helper/constructor/identity shapes |
| 2 | Interceptor chains + wrapper | HIGH | shared substrate, behavior-modifying, concurrency-adjacent |
| 3 | Shared helpers + behavioral contract | **HIGH** | **data integrity** (wrong order tears down live deps) **+ architectural** (must not become a hand-written engine, ADR-004) |
| 4 | `RunUntilSignal` | LOW-MEDIUM | concurrency + cross-platform signals; behavioral changes affect generated apps |
| 5 | Wire AST parsing | HIGH | couples to Wire output shape; must fail loudly |
| 6 | Level computation | **HIGH** | **correctness crux**: decides all ordering guarantees |
| 7 | Cleanup folding | **MEDIUM-HIGH** | ADR-008: cleanup funcs are the *primary* teardown mechanism; wrong position is a data-integrity risk on par with Phases 3/6/8, scoped down only because the change itself is narrow |
| 8 | Code emission | **HIGH** | constructor cleanup-discard (double-teardown/stranded-cleanup risk); naming stability |
| 9 | Generation driver | MEDIUM | user-facing CLI; clean failure modes |
| 10 | Drift check + integration | LOW-MED | flaky-drift hazard across environments |

**Key regression chains (re-verify the earlier phase when the later one
lands):**

- **Phase 7 → Phase 6.** Phase 6's `occupiesLevel` already gives a
  cleanup-only value a level of its own. Phase 7 only classifies the
  teardown form for every occupant: component, cleanup, or both. It adds
  or repositions nothing. Re-run all of Phase 6's level tests. Also
  re-run the generated behavioral tests and the goldens. A passing drift
  check alone does not prove the behavioral correctness of wrong output.
- **Phase 7 → Phase 3.** A folded cleanup changes teardown ordering.
  Re-run all ordering invariants against a fixture whose ordering depends
  on a folded cleanup's position.
- **Phase 5/6 change → generated behavior.** A parser or analysis change
  can commit wrong generated output that still passes the drift check.
  Drift only proves the output matches the committed copy. It does not
  prove the copy is correct. Always re-run the Phase 8 golden tests and
  the Phase 10 end-to-end behavioral suite. A passing drift diff alone is
  not enough.
- **Phases 1–4 → Phase 8.** Any runtime-contract change invalidates the
  golden files. Regenerate them, and review the diffs deliberately.
- **Phase 4 behavioral change → generated apps.** `RunUntilSignal`'s
  semantics can change without its signature changing. Re-run the Phase
  10 end-to-end suite, not only the golden diffs.
- **Phase 8/9 → Phase 1 API-surface golden.** Once generated symbols
  exist, re-run the Phase 1 API-surface golden. Confirm two things: the
  framework surface gained no new exports beyond the runtime-support
  package (ADR-010), and generated code compiles against those frozen
  surfaces only.
- **Metadata/observability across Phase 8.** Re-verify that three things
  survive into generated output: component identity (`FromContext`),
  operation identity (conveyed by the interceptor method, ADR-007), and
  duration-measurability. Run this as an integration check, so the
  metadata contract cannot silently regress at the generation boundary.
- **Phase 8 → Phase 3.** Generated code must satisfy Phase 3's behavioral
  suite. Run that suite against the generated output itself, not only
  against hand-written fakes.
- **Anything → Phase 10.** The drift check is the backstop. A diff there
  means one of two things: an intended regeneration, which you review and
  commit, or accidental drift, which you fix.

**Open decisions:** none blocking. Most design decisions are recorded in
the canonical docs: the PRD, ADR-001 through ADR-010, and the Architecture
document. A small number are plan-level defaults instead. Each fills a
genuine gap in those docs. None restates a documented decision. Each is
called out at its point of use, so it isn't mistaken for ADR policy. Three
defaults fall in this category. First, the overrun log sink (Phase 2), now
pinned to a concrete `slog` shape. Second, the panic recovery point (Phase
2/3). ADR-006 (Component Panics) fixes the recovery policy itself, for
both graph and boundary components. This plan only adds where recovery
runs. Third, the `internal/bridge` package (Phase 1/2), which Go's
visibility rules require but no ADR names. Separately, the
runtime-support-package sanity check at Phase 3's generated-shape pin
(ADR-010) remains a confirmed-in-flight default, as before.
