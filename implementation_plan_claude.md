# Yama v2 — Implementation Plan

**Target:** the compile-time lifecycle orchestration framework described in
`docs/PRD.md` and `docs/adr/ADR-001`…`ADR-010`, with the resolved architecture in
`docs/Architecture.md`.

**Audience:** a coding agent executing discrete, independently testable phases.

---

## 0. Orientation: what is being built, and what already exists

Yama v2 has **three separable artifacts**:

1. **A hand-written runtime library** (`package yama`) — the stable public API:
   the capability interfaces (`Starter`/`Quiescer`/`Stopper`), the interceptor
   interfaces, `ErrStartFailed`, `ComponentFromContext`, the boundary options
   (`WithBeginNode`/`WithEndNode`), and `RunUntilSignal`.
2. **A Yama-owned runtime-support package** (e.g. `l7e.io/yama/v2/yamart`, ADR-010)
   — the generic execution plumbing (chain construction, per-node wrapper,
   fail-fast level executor, boundary runner, `cleanupAdapter`, built-in overrun
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
  construction, per-node wrapper, fail-fast level executor, boundary runner,
  `cleanupAdapter`, the built-in overrun interceptor) lives in a Yama-owned sibling
  package (e.g. `yamart`) that generated code imports — exported so
  application-package code can call it, but not part of the stable ADR-007 API.
  Only graph-specific code (level structs + ordering methods, `YamaInterceptors`,
  `NewLifecycle`) is generated inline. This is a well-justified default; Phase 3's
  "pin the generated-code shape" step is the checkpoint to re-confirm it before
  Phase 8 bakes it into the emitter.
- **The overrun interceptor (Architecture §10/§20/§21).** Per-node deadline-overrun
  logging is a Yama-authored interceptor auto-attached to every node — internal, no
  public API. Its log sink is stdlib `log/slog` by default (Phase 2 pins this
  concretely; see Phase 2 for the exact checkable requirement).
- **A shared internal bridge package (`internal/bridge` or similar), needed by Go
  visibility rules, not named in any ADR — and its reach is bounded by a second Go
  rule that must not be conflated with the first.** `package yama`'s context-carrier
  key type is unexported (Phase 1), and `Lifecycle`'s fields must stay unexported to
  preserve encapsulation — but the per-node wrapper that needs to *set* both lives
  in the runtime-support package (ADR-010), a different package from `package
  yama`. The fix: put the shared bits (the context-key type, and the
  `Lifecycle`-construction primitive) in a small internal package under the module
  root, e.g. `internal/bridge`, that both `package yama` and the runtime-support
  package import. **Two distinct Go visibility mechanisms are both in play here,
  and the design leans on both correctly:**
  - **Import-path visibility** (the `internal/` directory convention): only code
    rooted at `l7e.io/yama/v2/...` may import `internal/bridge` at all. `package
    yama` and the runtime-support package (both inside that module) qualify.
    **Generated application code does not** — it lives in the *application's own,
    separate* Go module, and Go's `internal/` rule makes that import a hard compile
    error regardless of anything else in this design. Generated code, and any other
    external consumer, must reach this machinery exclusively through the
    runtime-support package's *exported* API (ADR-010) — it never imports
    `internal/bridge` directly, and this plan does not claim otherwise.
  - **Identifier export** (capitalization): the symbols inside `internal/bridge`
    that the runtime-support package calls (the setter/accessor pair, the
    `Lifecycle`-construction primitive) must themselves be **exported identifiers**
    (e.g. `bridge.SetComponent`, `bridge.Component`), not lowercase/unexported —
    a package permitted to import another package by path still cannot reach its
    unexported identifiers. The package's restricted *import path* is what keeps
    this out of reach of external modules; capitalization alone would not.
  - `package yama` re-exports `ComponentFromContext` and `Lifecycle`'s methods as
    thin public wrappers over `internal/bridge`'s exported functions; the
    runtime-support package imports the same internal package to attach identity
    and to assemble the `*Lifecycle` value it hands back to generated code through
    its own exported constructor. This keeps `package yama`'s literal exported
    surface exactly what ADR-007 wants, while giving the runtime-support package
    genuine, compiling access to the shared internals — and keeps generated,
    external-module code off of `internal/bridge` entirely, reaching only the
    runtime-support package's documented exported surface. Phase 1 introduces this
    package; Phase 2 consumes it; **Phase 9 adds the external-module compile
    fixture that proves it** — Phase 9's example app, in its own `go.mod` outside
    the Yama module, imports only `package yama` and the runtime-support package,
    proving generated code compiles from a genuinely external module and never
    touches `internal/bridge`.

Plan-level implementation choices not covered by the docs (by design): the
generator lives in `internal/generator` behind a thin `cmd/yama`; Google Wire is
pinned as a `go.mod` tool and invoked via `go tool wire`; v1 is deleted in Phase 0;
the `internal/bridge` package above; **the graph-participant panic policy**
(Phase 2/3 — the PRD and ADRs define panic handling only for boundary nodes,
ADR-009/Architecture §18, and are silent on panics from ordinary `Starter`/
`Quiescer`/`Stopper` graph participants or interceptors, so the recover-and-convert
policy below is a plan-level default consistent with ADR-006's "shutdown always
completes, no error aggregation" philosophy, not a documented ADR decision); and
**`RunUntilSignal`'s default signal set** (Phase 4 — ADR-007/Architecture give only
the signature `signals ...os.Signal` and say it "waits for the signal," without
specifying empty-variadic behavior; SIGINT/SIGTERM is a plan-level default). These
are stated in the phases that use them.

---

## Phase 0 — Repo disposition, module & tooling baseline

**Goal.** Remove the v1 signal-watcher (green-field rewrite) and establish the
v2 package skeleton and tooling so later phases have a clean, compiling home.

**Files/modules likely touched.** Delete `yama.go`, `option.go`, and the v1 tests
(`yama_test.go`, `yama_unix_test.go`, `yama_windows_test.go`, `example_unix_test.go`);
touch `go.mod`, `go.sum`, new package dirs, `README.md`, possibly `docs/adr/` (a
short new ADR recording the v1→v2 disposition — the next free number, **ADR-011**;
ADR-010 is already taken by the runtime-support-package decision).

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
component-identity context carrier + `ComponentFromContext`, and the *signatures*
(bodies may be stubs) of **all** public helpers — `WithBeginNode`/`WithEndNode`,
`RunUntilSignal` — plus the generated `YamaInterceptors` shape (per ADR-007 +
Architecture §13). After this phase the API-surface golden is **complete**; Phases
3/4 add behavior to these symbols without changing their signatures.

**Files/modules touched.** `lifecycle.go` (the `Lifecycle` struct + `Start`/`Stop`
method signatures, the capability interfaces, `ErrStartFailed`), `interceptor.go`,
`context.go` (unexported key type + accessor), `options.go` (boundary option
types), `helpers.go` (helper signatures + stub bodies), and
`internal/bridge/` — a small internal package, importable **only** by code
rooted at `l7e.io/yama/v2/...` (Go's `internal/` import-path rule), which covers
`package yama` and the Phase 2 runtime-support package but **not** generated
application code (a separate module — see Orientation §0 for why that boundary
matters and why this package's own symbols must still be exported identifiers,
not lowercase, despite the path restriction). `internal/bridge` holds the
context-key type and the `Lifecycle`-construction primitive, both exported
identifiers within that path-restricted package (see Orientation §0, "shared
internal bridge package"). `package yama`'s `context.go` and `lifecycle.go` become
thin exported wrappers over `internal/bridge`; this is what lets the Phase 2
wrapper (in a different in-module package) legally set the same identity the
Phase 1 accessor reads. Generated code (Phase 8/9) never imports
`internal/bridge` — it reaches `Lifecycle` construction only through the
runtime-support package's own exported, ADR-010-sanctioned API, which is what
keeps `package yama` from exposing a public, arbitrary-construction API that would
violate ADR-007's minimal-surface intent.

**Dependencies.** Phase 0.

**Risk.** **HIGH — external-facing API.** (Re-rated up from an earlier MEDIUM: the
task's own criteria classify external-facing APIs as high-risk, and this phase
*is* the permanent public surface. Every symbol is a long-term compatibility
commitment (ADR-007); a wrong signature forces a breaking change and invalidates
every downstream golden.) (Constructor/helper signatures follow ADR-007 +
Architecture §13; the ADR-010 runtime-support symbols live in a separate package,
so they do not enter `package yama`'s frozen surface here.)

**Definition of Done.**
- *Process:*
  - Read ADR-003, ADR-005, ADR-006, ADR-007 **and Architecture §12** in full before
    writing signatures; the non-uniform interceptor shapes are a deliberate,
    explicitly-argued decision (Start returns `error`; Quiesce/Stop do not) and must
    not be "normalized."
  - Include `ComponentFromContext` in the public surface (ADR-007 §"Public Context
    Accessor"); the operation is conveyed by the interceptor method, so do **not**
    add an operation-identity context accessor.
  - Do **not** add any symbol outside the public surface enumerated in
    Architecture's Appendix C (Public API Reference) — the concrete target this
    phase builds — which reflects the ADR-007 minimal-API decision (no `Graph`,
    `Node`, `Plan`, `Level`, `Register`, `SetLogger`, config types). Enforce with a
    committed **API-surface golden test** (e.g. `go doc`-style exported-symbol
    snapshot) that fails on any unlisted export.
- *Output checks:*
  - **`Lifecycle` exists as a concrete exported struct with exactly
    `(*Lifecycle) Start(context.Context) error` and `(*Lifecycle) Stop(context.Context)`**
    (Architecture Appendix C, ADR-007 §"Public Lifecycle Type"); `Quiesce` is
    **not** exposed on `Lifecycle` — asserted by the API-surface golden showing no
    `Quiesce` method on the type. `Lifecycle`'s fields are unexported; the only way
    to obtain a populated `*Lifecycle` is through the `internal/bridge` package
    (below), never through a public zero-value construction path — a stub-bodied
    `Lifecycle` with unset internal state is acceptable for this phase (Phase 3
    gives it real behavior), but the **type and method signatures** are frozen now.
  - **The `internal/bridge` compiles and is reachable from a second,
    sibling *in-module* test package** (standing in for the Phase 2 runtime-support
    package, which does not exist yet): a fixture package under the same module
    (`l7e.io/yama/v2/...`) imports `internal/bridge`, attaches a
    component-identity value using its exported setter, and
    `yama.ComponentFromContext` — reading through `package yama`'s thin wrapper —
    recovers the same value. This proves the cross-package identity path Phase 2
    depends on actually compiles before Phase 2 starts, rather than discovering the
    visibility problem mid-Phase-2.
  - **The negative case is asserted too, not just the positive one:** a fixture
    module with its own separate `go.mod` (outside `l7e.io/yama/v2`) that attempts
    `import "l7e.io/yama/v2/internal/bridge"` **fails to compile** with Go's
    standard `internal/` visibility error. This is a one-time proof that the module
    boundary is real, not assumed — cheap to add here, and it is what the DoD in
    Phase 9 (the external-module compile fixture, added below) later exercises
    positively by construction rather than by accident.
  - `Starter.Start(context.Context) error`, `Quiescer.Quiesce(context.Context)`,
    `Stopper.Stop(context.Context)` exist with exactly these signatures.
  - `StartInterceptor.Start(ctx, next Starter) error`,
    `QuiesceInterceptor.Quiesce(ctx, next Quiescer)`,
    `StopInterceptor.Stop(ctx, next Stopper)` exist; Quiesce/Stop interceptors
    return nothing. The API-surface golden is what pins these shapes: it renders
    the interface method signatures, so normalizing Quiesce or Stop to return an
    error fails it with a reviewable diff. Do **not** add a "conformance table" of
    assertions against witness types written solely to satisfy them — a witness
    that exists only to be asserted proves nothing, and its compile error is
    cleared by editing the witness. Compile-time interface guards belong where a
    real implementation exists for its own reasons (Phase 2's overrun interceptor
    and chain fixtures: `var _ StartInterceptor = (*overrunInterceptor)(nil)`).
  - `ErrStartFailed` is a package-level `error`; `errors.Is(x, ErrStartFailed)`
    works.
  - The component-identity type returned by `ComponentFromContext` is **concretely
    defined** here, not left as "a small value": specify its exact shape and fields
    (proposed default: an exported value type carrying the generated participant
    **name** as a string, e.g. `type Component struct { Name string }`, plus an `ok
    bool` return). Whatever is chosen is fixed now and named in the DoD.
  - `ComponentFromContext(ctx)` returns that identity when previously attached and a
    documented zero value + `ok=false` when absent. The context key is an
    **unexported** type (collision-proof) — verified by a test that a caller's own
    `context.WithValue(ctx, "component", …)` does **not** leak into the accessor.
  - Accessor exposes component metadata **only** — no graph/plan/error leakage, and
    **no operation identity** (that is conveyed by which interceptor method runs,
    per Architecture §12 / ADR-007); asserted by the returned type having exactly the
    fields defined above.
  - **All public-helper signatures are declared and compile** (`WithBeginNode`,
    `WithEndNode`, `RunUntilSignal`) with stub bodies (e.g. `panic("unimplemented")`
    guarded so it never ships, or a documented no-op). The API-surface golden
    includes them, so it is **complete at the end of Phase 1** and does not grow in
    Phase 4.
- *Edge/failure cases:* `ComponentFromContext` on a nil/background context returns
  the documented absent result (`ok=false`), never panics.

**Regression note.** These signatures are consumed by Phases 2–10. Any change here
after Phase 5 forces regeneration and re-verification of all golden files
(Phase 8/10). Freeze early. The API-surface golden frozen here is re-verified (not
extended) after Phases 4, 8, and 9. A change to `internal/bridge` (the bridge)
forces re-running both this phase's cross-package fixture check and Phase 2's
identity-attachment tests, since both depend on the same shared type.

---

## Phase 2 — Interceptor chains + universal wrapper (runtime core)

**Goal.** Implement the runtime machinery that generated code will call: build the
three separate operation chains (global + per-participant, registration-ordered),
attach component identity to context before the chain runs, and the **universal
per-node wrapper** that gives every node per-node attribution (identity in context)
and threads the caller's context — *with its deadline* — unchanged.

**Files/modules touched.** chain builders, the per-node wrapper, and identity
attachment — in the **runtime-support package** (ADR-010; the package path, e.g.
`l7e.io/yama/v2/yamart`, is ADR-010's own illustrative example, not a binding name —
pick and record the real module-qualified path here), exported so generated
app-package code can call them, and documented as not part of the stable ADR-007
API. Identity attachment goes through the **`internal/bridge` introduced
in Phase 1** — the wrapper imports it directly and calls its exported setter to
attach the same unexported-typed context value `yama.ComponentFromContext` reads;
it does **not** invent a second, incompatible key. All exercised via hand-authored
"as-if-generated" fixtures in `_test.go`.

**Wrapper vs. level runner (component boundary).** Phase 2 and Phase 3 share
ownership of per-node execution, so the split is stated explicitly to avoid the two
being conflated: the **per-node wrapper** (this phase) is a single function/closure
around one node's operation — it attaches identity, threads context, and runs the
interceptor chain, and it is the unit Phase 8's generated per-node calls invoke
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
    **Yama-authored interceptor, auto-attached to every node** (internal, no exported
    API); the wrapper only attaches component identity and threads the caller's
    context (deadline intact). **Log sink, pinned as a concrete, checkable
    requirement (plan-level default; not specified by the docs):** the interceptor
    logs via the stdlib `log/slog` default logger at `Warn` level, one line per
    overrun, including the component name and the elapsed-vs-deadline delta; it
    takes no constructor argument to redirect or silence the sink in this phase. A
    test asserts an overrun produces exactly one `slog` record matching that shape;
    the "can it be silenced" question is deferred, out of scope, and not claimed as
    done here. See the overrun check below.
  - **Panic policy — a plan-level default filling a documented gap (see Orientation
    §0), defined and owned by Phase 3** (recovery happens at the
    operation/level-runner boundary, not inside the chain). Phase 2 only asserts
    that the chain/wrapper do not *themselves* recover or swallow a panic — they let
    it reach the Phase 3 boundary. The single exact policy: **Start panics are
    recovered at the Phase 3 boundary and converted to a start failure
    (`ErrStartFailed`); Quiesce/Stop panics are recovered there and swallowed so the
    traversal completes.** An interceptor that wants to *observe* a panic wraps
    `next` with its own `recover`. (No panic "propagates to the caller" and also
    "becomes `ErrStartFailed`" — that was contradictory; conversion requires
    recovery, which is what happens.)
  - Every node is wrapped whether or not an interceptor is attached (universal
    wrapping is not opt-in) — assert this explicitly.
- *Output checks:*
  - Chain execution order equals registration order: given `[Telemetry, Metrics,
    Logging]` the observed call order is Telemetry→Metrics→Logging→component
    (Architecture §10). A test asserts the exact order string.
  - Global interceptors apply to all participants; a per-participant interceptor
    applies only to its participant and to no other (two-participant fixture
    proves isolation).
  - Only interceptors implementing the operation-specific interface join that
    operation's chain (a type implementing only `StartInterceptor` never runs in
    the Stop chain).
  - An interceptor can: (a) observe, (b) modify context seen by `next`
    (`ComponentFromContext` and a custom value both visible downstream),
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
  - **The built-in overrun interceptor reports per-node overrun (Architecture
    §20/§21).** With a
    context whose deadline fires before a node's operation returns, the
    Yama-authored auto-attached interceptor reports the overrun **exactly once**,
    attributed via `ComponentFromContext(ctx)`, to the chosen sink (assert against a
    test sink). It does **not** cancel or abandon the operation; the wrapper waits
    for `next` to return (asserted with a controllable slow fake). No exported
    overrun API is involved.
  - Chains are built **once** at construction and reused; the wrapper does not
    rebuild chains per call (asserted via a build-counter).
- *Edge/failure cases:* empty interceptor set → node still invoked exactly once;
  nil per-participant field → no panic.

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
on — the per-node wrapper (Phase 2), a boundary runner, an `errgroup`-style
intra-level concurrency helper with fail-fast, per-participant "started" tracking,
and the quiesce-then-teardown shutdown/cleanup plumbing — and establish the
**behavioral contract** (ordering invariants, fail-fast, boundaries) that Phase 8's
*generated* code must later satisfy. This phase does **not** build a generic runtime
runner that interprets a plan; per ADR-004 and ADR-010, the level structs and their
Start/Quiesce/Stop orchestration methods are generated, not hand-written here.

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
    the "designing around fixtures" concern).** Before writing any fixture, produce a
    short written skeleton of what a generated level struct and its
    Start/Quiesce/Stop methods will look like (the shape Phase 8 emits). The
    hand-authored test stand-ins in this phase MUST match that skeleton, so Phase 8
    can later emit code of the same shape and inherit this phase's behavioral suite.
    **Commit this skeleton as a checked-in artifact, not only prose**: a short
    `.go` template/example file (e.g. `internal/generator/testdata/skeleton.go.example`,
    excluded from build) that Phase 8's golden-emission tests diff their *shape*
    against (field names/order, method signatures — not byte-identical, since real
    graphs vary). This turns "Phase 8 conforms to Phase 3's skeleton" from an
    implicit expectation enforced only by shared behavioral tests into a checkable
    artifact: if Phase 8 drifts from the pinned shape, the shape-diff fails
    independently of whether the behavioral suite happens to still pass.
  - **Guard against a generic engine (ADR-004 process check):** no exported or
    unexported type in `package yama` may hold a runtime list/plan of levels that a
    hand-written loop interprets. Orchestration flows through generated methods
    calling shared helpers. Verify by inspection and note it in the phase writeup.
  - Run the full test suite under `-race` for every check below; a passing but
    racy implementation is not done.
  - Do not introduce any timeout/deadline of Yama's own — the only deadline is the
    caller's context (ADR-003 §"Stop Deadline", Architecture §20). Assert no
    `context.WithTimeout` with a framework constant exists (grep/process check).
  - **Boundary option ownership:** `WithBeginNode`/`WithEndNode` *signatures* are
    fixed in Phase 1; their *behavior* is implemented and tested here. Phase 4 does
    **not** revisit them (removes the earlier "finalize in Phase 4" overlap).
- *Output checks (ordering invariants):*
  - **Invariant 1:** a dependency's `Start` completes before any dependent's
    `Start` begins (fixture with A→B→C asserts start order and that same-level
    independent nodes overlap in time).
  - **Invariant 2:** a dependent's `Stop` completes before its dependency's `Stop`
    begins (reverse order asserted).
  - **Invariant 3:** *every* participant's `Quiesce` completes before *any*
    participant's teardown `Stop` begins (a global "phase" observer proves no
    interleave).
  - Quiesce runs in **reverse dependency order** (dependents quiesce first), same
    direction as Stop — and ordering **holds transitively through non-`Quiescer`
    nodes** (A→(non-quiescer)B→C: A quiesces before C).
  - Independent branches start/quiesce/stop **concurrently** (timing or
    barrier-based proof, not just "no error").
  - Nodes implementing only some capabilities: a `Starter`-only node never gets
    `Quiesce`/`Stop`; a `Stopper`-only node never gets `Start`.
- *Output checks (fail-fast + cleanup):*
  - Startup failure in the active level: startup context is canceled, no later
    level is started (asserted: level-N+1 nodes' `Start` never called), in-flight
    same-level ops are awaited to settle, and **only the successfully-started**
    participants are quiesced+stopped (scoping proven with a node that failed vs.
    a sibling that started).
  - The cleanup path is **the same code** as normal `Stop` (Invariant 4) — proven
    by a shared observer seeing identical ordering, not a parallel implementation.
  - `Start` returns exactly `ErrStartFailed` (via `errors.Is`) and never the
    component's error; `Stop` returns nothing.
  - **`Stop` is idempotent** (this is the framework's own guarantee that makes a
    separate once-only helper — informally called `EnsureExactlyOnce` in this plan,
    but never a documented ADR symbol, see Phase 4 — unnecessary): calling `Stop`
    more than once, or
    concurrently, runs the quiesce+teardown passes **once**; later/overlapping calls
    observe the same completion and re-trigger nothing. Race-tested. (Applies too
    when `Start` already ran startup-failure cleanup, then the app also calls
    `Stop`.)
  - **Start deadline exceeded → `ErrStartFailed` (PRD §6.9 / ADR-006 §"Timeout
    Errors").** A participant whose `Start` exceeds the caller's context deadline is
    handled as an ordinary start failure: `Start` returns `ErrStartFailed` (not a
    distinct timeout error), the startup context is canceled, later levels are not
    started, and startup-failure cleanup (quiesce + teardown) runs over the
    successfully-started participants. Asserted with a fixture whose one participant
    blocks past a short caller deadline.
  - **Caller context already canceled at `Start`** (explicit expected result, was
    previously unspecified): `Start` performs no participant starts beyond what the
    cancellation permits and returns `ErrStartFailed`; any participant that did
    start is cleaned up via the normal shutdown sequence. Asserted.
  - A hung participant stalls the traversal (does **not** return early) — a test
    with a bounded wait confirms the traversal blocks on it and later nodes have
    not run; document that only SIGKILL bounds this (do not add a framework
    escape).
  - **Panic policy — one exact rule, recovery at the operation/level-runner
    boundary (resolves the review's inconsistency):**
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
  - Begin nodes run before all graph nodes in each pass they join; end nodes after
    all graph nodes — for **all three** passes (Start / Quiesce / Stop) and also in
    startup-failure cleanup (same passes, no separate path).
  - A boundary node joins a pass only if it implements that pass's interface.
  - Boundary execution is **best-effort**: a begin node that errors/panics does not
    stop the graph pass; an end node that errors/panics does not change the
    outcome (asserted for both error and panic).
  - Boundary sets are unordered/concurrent (no ordering assertion is made or
    required; a test must not depend on intra-set order).
- *Edge/failure cases:* zero participants; a single participant with all/none
  capabilities; caller context canceled mid-shutdown (passes still run to
  completion, return nothing); no goroutine leaks (goleak-style check or explicit
  wait-group accounting). (The canceled-at-`Start` and start-deadline cases are
  covered as first-class output checks above.)

**Regression note.** Phase 7 inserts `cleanupAdapter` `Stopper`s into shutdown
levels — re-run **all** Phase 3 ordering tests after Phase 7 to confirm adapters
occupy the correct DAG position and don't perturb ordering. Phase 8's emitted code
must produce level structures that satisfy these same invariants — Phase 3's tests
become the behavioral contract Phase 10 re-checks on generated output.

---

## Phase 4 — Public helper: `RunUntilSignal`

**Goal.** Implement the **body** of `RunUntilSignal`. Its **signature is already
frozen in Phase 1** (and in the Phase 1 API-surface golden); this phase changes
behavior, not signature. (`WithBeginNode`/`WithEndNode` are fixed in Phase 1 and
implemented in Phase 3 — not revisited here. **`RunInBackground` and
`EnsureExactlyOnce` are plan-level names for helpers this plan does not build, not
a documented ADR-007 decision** — no canonical doc names either helper. ADR-007
does establish the *reasoning* that makes them unnecessary: a `Start` that would
otherwise block is the component's own responsibility to background (ADR-007
§"Public Helpers"), and `Stop`'s idempotency is the framework's own guarantee
(Phase 3), so no once-only helper is needed. The two names above are only this
plan's shorthand for "the helpers that reasoning replaces," not framework symbols
ever slated for implementation.)

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
    (default `SIGINT`/`SIGTERM` when none passed — **a plan-level default; ADR-007
    and Architecture specify only the `signals ...os.Signal` signature and do not
    define empty-variadic behavior**, so this default is stated here rather than
    left implicit), then calls `Stop` and returns; returns `ErrStartFailed` if
    `Start` failed (and does **not** wait for a signal in that case). Signal
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
root-struct manifest, and detected legacy cleanup functions — **without** yet
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
  - **Every** new-variable creation form is captured as a node: call-expression
    providers, `wire.Value`/`InterfaceValue` assignments, and
    `wire.Struct`/`FieldsOf` struct literals (one fixture per form; each asserts
    the node is present).
  - The final returned root-struct literal (e.g. `App`) is recorded as the
    manifest of roots and is **not** itself a node.
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
  - Empty injector (no lifecycle-capable nodes) parses cleanly and yields an empty
    node set.

**Regression note.** This phase's output shape (node/edge model) is consumed by
Phases 6–8. Freeze the internal node representation before Phase 6. Wire version
bumps can change output shape — Phase 10's drift check is the guard; note the
tested Wire version.

---

## Phase 6 — Generator: lifecycle capability detection + level computation

**Goal.** From the parsed graph, determine each node's lifecycle capabilities and
compute the three level structures: startup levels (dependency order), and quiesce
& shutdown levels (reverse dependency order), with **transitive ordering preserved
through dependency-only / non-capable nodes**.

**Files/modules touched.** `internal/generator/analyze*.go`,
`internal/generator/levels*.go`, fixtures + expected-level assertions.

**Dependencies.** Phases 1 and 5. (Phase 5 supplies the parsed node/edge model;
Phase 1 supplies the capability interface definitions that capability detection
resolves against — "implements `Starter`" is decided by static type analysis of the
generated package, not runtime reflection.)

**Risk.** **HIGH — the correctness crux of the entire project.** This is where the
ordering guarantees the runtime relies on are actually *decided*. A wrong level
assignment yields silently-wrong startup/shutdown ordering in every consuming app.

**Definition of Done.**
- *Process:*
  - Read ADR-003 §"Startup/Quiesce/Stop Semantics" and Architecture §7 before
    implementing; the transitive-through-non-capable-node rule is stated in both
    and is the most error-prone requirement.
  - Capability detection is done by **static type analysis** of the generated
    package (does the concrete type satisfy `Starter`/`Quiescer`/`Stopper`?), with
    **no reflection** and no runtime discovery (ADR-001, PRD §4).
- *Output checks:*
  - Startup levels: nodes with no lifecycle ordering edge between them share a
    level (may run concurrently); a dependent is in a strictly later level than
    its dependency (the diamond `DB → {Router, Worker}` yields `[DB]`, then
    `[Router, Worker]` — matching ADR-003's worked example).
  - Shutdown & quiesce levels are the **reverse**: `[Router, Worker]` then `[DB]`.
  - **Transitive ordering through non-capable nodes:** for capable A → non-capable
    B → capable C, A and C remain correctly ordered relative to each other in all
    three level sets (dedicated fixture; this is the headline edge case).
  - Only capability-bearing nodes appear in a given operation's levels
    (dependency-only nodes influence ordering but receive no callbacks); a
    `Quiescer`-only-absent node is skipped in quiesce levels but still transmits
    ordering.
  - Level computation is **deterministic**: identical input yields byte-identical
    level assignment across runs (stable node iteration order — asserted by
    repeated runs), so goldens in Phase 8 are stable.
- *Edge/failure cases:* a node implementing all three vs. exactly one capability;
  a graph with no capable nodes (empty level sets, no error); a wide fan-out
  (many independent nodes in one level); a deep chain (many single-node levels);
  a node that is both depended-upon and a dependent (interior of a chain).

**Regression note.** Adding `cleanupAdapter` nodes (Phase 7) injects synthetic
`Stopper`s into the shutdown/teardown level computation "at the cleaned-up value's
position." Re-run all Phase 6 level tests after Phase 7 to confirm adapters land in
the right level and don't shift other nodes. This is the highest-value regression
checkpoint in the plan.

---

## Phase 7 — Generator: cleanup-adapter synthesis

**Goal.** A cleanup function is shorthand for a `Stopper`: wrap **every** Wire
cleanup function in a synthetic `cleanupAdapter` `Stopper` positioned at the
cleaned-up value's DAG slot (ordering only, type graph unmodified). Structurally
this is a small, well-scoped phase — essentially adapter synthesis + positioning,
and it could fold into Phase 6/8 mechanically — but its output is **not** low-
stakes: ADR-008 states cleanup functions are "the primary graceful-shutdown
teardown mechanism, not only as error-path cleanup," so a mispositioned or dropped
adapter here has the same failure mode Phase 3 rates HIGH for ("a dependency torn
down while a dependent still uses it") in exactly the code path most real
applications' actual resource teardown (DB pools, file handles, connections) runs
through. Kept separate to hold the cleanup semantics and the
both-a-`Stopper`-and-a-cleanup case in one place. Background: ADR-008
§"Cleanup functions".

**Files/modules touched.** `internal/generator/cleanup*.go`, the `cleanupAdapter`
type (location/visibility per ADR-010), fixtures covering plain-cleanup and
both-a-`Stopper`-and-a-cleanup values.

**Dependencies.** Phases 5–6.

**Risk.** **MEDIUM-HIGH (re-rated up from MEDIUM per review)** — a wrong adapter
position reintroduces the ordering bug the project exists to prevent, and per
ADR-008 this is the *primary* teardown mechanism for most applications, not a
secondary one. This is data-integrity risk in the same class as Phases 3, 6, and 8
(wrong shutdown ordering / broken cleanup corrupts real applications), scoped down
from those slightly only because the change here is narrow and mechanical (adapter
positioning, not level computation or constructor semantics) and easier to get
right by construction. The DoD below is held to that standard.

**Definition of Done.**
- *Process:*
  - Read ADR-008 §"Cleanup functions" (as amended) and Architecture §4 step 6, §17.
- *Output checks:*
  - Every detected cleanup function becomes a `cleanupAdapter` implementing
    `Stopper`, inserted at the **same DAG position** as the value it cleans up, so
    Phase 6's shutdown levels place it correctly (verified against Phase 3's
    ordering invariants on the generated result). (`cleanupAdapter`'s
    location/visibility follows ADR-010, like the other shared helpers — it must be
    constructible from generated app-package code.)
  - Teardown has a **single dispatch path**: hand-written `Stopper`s and
    adapter-wrapped cleanups are treated identically (no special-casing in emitted
    code — asserted by inspecting the golden output).
  - The injected **type graph is unmodified** — adapters exist only for ordering
    (assert the app's own types are untouched).
  - **Both-a-`Stopper`-and-a-cleanup value:** a provider that returns a cleanup func
    *and* whose value implements `Stopper` yields **two `Stopper` nodes at the same
    DAG position** — the value and the adapter — sharing incoming/outgoing edges,
    landing in the **same teardown level**, running concurrently with no ordering
    between them. Asserted; **not** an error and **not** deduplicated.
- *Edge/failure cases:* a value with a cleanup func but no other capability (adapter
  is its only lifecycle role); multiple cleanup funcs in one injector (each adapted
  at its own position); a `Stopper` value with no cleanup func (plain `Stopper`, no
  adapter).

**Regression note.** See Phase 6 note — this phase is the reason Phase 6's tests
must be re-run. Also re-run Phase 3 ordering tests against a fixture whose ordering
depends on an adapter's position, and against a both-a-`Stopper`-and-a-cleanup
fixture (two teardown nodes, same level).

---

## Phase 8 — Generator: code emission, naming, formatting

**Goal.** Emit `lifecycle_gen.go`: the provenance header, generated level structs,
per-node wrappers, interceptor-chain construction, the `YamaInterceptors` input
and per-participant fields, the generated `NewLifecycle`-style constructor
returning `(*App, *Lifecycle, error)`, and the Start/Quiesce/Stop methods —
gofmt-clean and deterministic.

**Files/modules touched.** `internal/generator/emit*.go`,
`internal/generator/templates` (or `go/ast`/`jennifer`-style builder — decide),
golden fixtures `…/testdata/*.golden`.

**Dependencies.** Phases 1–7 (needs the runtime contracts *and* the analysis).

**Risk.** **HIGH** (re-rated up from MEDIUM per review). Phase 6 decides *ordering*,
but this phase owns **constructor semantics that are themselves data-integrity
sensitive**: capturing-and-discarding Wire's raw cleanup func so teardown runs only
via `Stop` — a mistake here **double-tears-down or strands cleanup**. Naming
stability and deterministic collision resolution also live here. The DoD below is
deliberately as detailed as a high-risk phase warrants.

**Definition of Done.**
- *Process:*
  - Read ADR-004, ADR-007, Architecture §8, §13, §14, and Appendix C before
    emitting; keep every generated implementation symbol in the `yama`-prefixed
    private namespace and keep the public surface exactly matching Architecture's
    Appendix C (Public API Reference).
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

**Regression note.** Golden files are coupled to *both* the runtime API (Phases
1–4) and analysis (Phases 5–7). Any change upstream regenerates goldens — a golden
diff is expected and must be reviewed, not blindly accepted. Add a comment in the
golden test explaining that a diff means "confirm the change was intended."

---

## Phase 9 — Generation driver: one command, two files, `go:generate`

**Goal.** Wire the pieces into a single user-facing generation step: one
`go:generate` directive and one command that runs `wire gen` then Yama's
generator, producing both `wire_gen.go` and `lifecycle_gen.go` in the target
package.

**Files/modules touched.** `cmd/yama/main.go`, example app under `examples/…`
with a `//go:generate` directive, README usage section. **The example app is its
own separate Go module** (its own `go.mod`, requiring `l7e.io/yama/v2` as a normal
external dependency — not a subpackage sharing Yama's `go.mod`). This is not
cosmetic: it is what makes the app "generated application code living outside the
Yama module" a fact of the build rather than an assumption, and it is the fixture
the Orientation §0 external-module compile proof (for `internal/bridge`) runs
against. **This only proves anything if the build actually compiles against the
checked-out tree, not a published/cached version of `l7e.io/yama/v2`** — a plain
`require l7e.io/yama/v2 vX.Y.Z` in the example's `go.mod` would let `go build` in
CI silently resolve some other, possibly stale, version from the module proxy
while the PR's actual changes go untested. The example's `go.mod` therefore
carries `replace l7e.io/yama/v2 => ../..` (or the repo uses a root-level `go.work`
with `use . ./examples/…` instead — either is acceptable, but one of them must be
present) pinning resolution to the local checkout unconditionally.

**Dependencies.** Phases 5–8.

**Risk.** MEDIUM — user-facing CLI/build ergonomics; must fail cleanly when the
target package is malformed.

**Definition of Done.**
- *Process:* read Architecture §4, §14 and ADR-008 §"Generation and drift".
- *Output checks:*
  - A single command run in a fixture app produces **both** files; running it
    twice is idempotent (second run yields no diff).
  - The `//go:generate` directive (using `go tool wire`) is documented and
    works via `go generate ./…` on the example app.
  - Generation is atomic-ish: a failure in the Yama step does not leave a
    half-written `lifecycle_gen.go` that would compile-break the package
    (write-to-temp-then-rename or equivalent; asserted).
  - **External-module compile proof (resolves the Orientation §0 concern for
    `internal/bridge`):** with the example app in its own `go.mod` (above),
    `go build ./...` and `go vet ./...` succeed for the example app *as a standalone
    module* (built from its own directory). **The DoD explicitly includes verifying
    the `replace` (or `go.work`) points at the local checkout**, not merely that the
    build passes — a build that silently resolved a stale published version would
    also "pass" while proving nothing. Verify by asserting the resolved module path
    in `go list -m l7e.io/yama/v2` (or `go env GOWORK` when using a workspace)
    reflects the local directory during this build, and additionally by a
    change-detection check: touching a file inside `package yama` or the
    runtime-support package and re-running the example app's build must be
    observable in that build (e.g. a deliberately broken signature in local Yama
    code makes the example app's build fail) — proving CI is actually compiling
    today's checkout, not a cached artifact. This is the actual proof that
    `lifecycle_gen.go`, compiled from a genuinely external module against the
    current code, reaches `Lifecycle` construction only through `package yama` and
    the runtime-support package's exported API — because if generated code ever
    accidentally referenced `internal/bridge`, this build would fail with Go's own
    `internal/` visibility
    error, not a Yama-authored check.
- *Edge/failure cases:* `wire` tool unresolvable (e.g. not pinned in `go.mod`) →
  clear toolchain error, distinct from a Wire *input* error; target package with no
  injector → clear error; read-only output dir → clear error.

**Regression note.** The example app becomes a living integration fixture reused by
Phase 10. Keep it minimal but covering ≥2 capabilities, a dependency-only node, and
one cleanup func.

---

## Phase 10 — CI drift check, integration & golden coverage, docs

**Goal.** Add the CI drift check (regenerate both files, diff against committed
copies, fail on divergence), an end-to-end integration test that builds and runs
the example app's lifecycle, and finalize docs/README.

**Files/modules touched.** `.github/workflows/ci.yml`, `Makefile`/scripts,
`examples/…` integration test, `README.md`, possibly `docs/`.

**Dependencies.** Phases 1–9.

**Risk.** LOW-MEDIUM — internal tooling and CI; the main hazard is a flaky or
environment-sensitive drift check (Wire version, gofmt version, OS line endings).

**Definition of Done.**
- *Process:* read Architecture §24 (testing strategy) and ADR-008 §"Generation and
  drift"; pin the Wire version used in CI and record it.
- *Output checks:*
  - CI regenerates `wire_gen.go` + `lifecycle_gen.go` for the example app and
    **fails** if either differs from the committed copy.
  - The drift mechanism itself is verified by a **durable, committed self-test**
    (script or `go test`), not a one-off manual tamper-and-revert: the test writes a
    known perturbation to a *copy* of a generated file, runs the drift comparison,
    asserts it reports a failure, and cleans up — so the guard is proven to actually
    catch drift on every CI run, not just once during review.
  - Drift check is stable across the CI matrix where it runs (pin OS/Go/Wire; run
    the drift job on a single well-defined platform to avoid gofmt/line-ending
    noise — do **not** run it across all three OSes unless proven stable).
  - End-to-end: the example app starts (dependency order observed), receives a
    signal / explicit `Stop`, quiesces-before-teardown, and shuts down in reverse
    order — asserted via an interceptor-based observer, under `-race`.
  - Public error contract: an integration-level test confirms callers receive only
    `ErrStartFailed` from `Start`, `Stop` returns nothing, and component errors are
    visible **only** through interceptors.
  - README documents: the `go:generate` line, the `(app, lifecycle, err)` pattern,
    `RunUntilSignal` as the typical `main`, and the boundary-node options.
  - **Large-graph generation is exercised (PRD §9 "Generated Code Complexity").**
    The example app used elsewhere in this phase is deliberately minimal, so it does
    not exercise this named project risk. Add one additional, separate fixture (not
    part of the example app or the drift check) with a wide/deep synthetic graph —
    on the order of 100+ nodes — that runs through the full pipeline (`wire
    generate` → Yama generation) and asserts: (a) generation completes in bounded
    time (a generous CI-stable ceiling, not a tight benchmark), (b) the emitted file
    still `gofmt`-cleanly compiles, (c) generated names stay unique/collision-free at
    that scale. **Concrete, checkable placement (not "some cadence"):** this is a
    named CI job (e.g. `large-graph` in `.github/workflows/ci.yml`) triggered on a
    weekly schedule (`schedule:` cron trigger) plus manually via
    `workflow_dispatch`, separate from the per-commit job so it never blocks normal
    merges. Its DoD is satisfied when that job definition exists in the committed
    workflow file and has been observed to pass at least once — not by an
    undocumented expectation that someone runs it "sometime."
- *Edge/failure cases:* drift check when `wire` version differs (documented,
  version pinned); coverage job (existing Coveralls step) still runs against the
  new packages.

**Regression note.** This phase locks the whole system. Any later change to
runtime API, analysis, or emission will trip the drift check — that is the intended
behavior. Document the "regenerate + review the diff" workflow so a legitimate
change isn't mistaken for accidental drift, and vice-versa.

---

## Appendix: risk summary & regression map

| Phase | Area | Risk | Why |
|------|------|------|-----|
| 0 | Repo/module disposition | LOW | mechanical; removes v1 (green-field rewrite) |
| 1 | Public API surface | **HIGH** | external-facing API — permanent compatibility commitment; freezes `Lifecycle`, helper/constructor/identity shapes |
| 2 | Interceptor chains + wrapper | HIGH | shared substrate, behavior-modifying, concurrency-adjacent |
| 3 | Shared helpers + behavioral contract | **HIGH** | **data integrity** (wrong order tears down live deps) **+ architectural** (must not become a hand-written engine, ADR-004) |
| 4 | Public helpers | MEDIUM | concurrency + cross-platform signals; behavioral changes affect generated apps |
| 5 | Wire AST parsing | HIGH | couples to Wire output shape; must fail loudly |
| 6 | Level computation | **HIGH** | **correctness crux** — decides all ordering guarantees |
| 7 | Cleanup-adapter synthesis | **MEDIUM-HIGH** | ADR-008: cleanup funcs are the *primary* teardown mechanism — wrong adapter position is data-integrity risk on par with Phases 3/6/8, scoped down only for narrowness of change |
| 8 | Code emission | **HIGH** | constructor cleanup-discard (double-teardown/stranded-cleanup risk); naming stability |
| 9 | Generation driver | MEDIUM | user-facing CLI; clean failure modes |
| 10 | Drift check + integration | LOW-MED | flaky-drift hazard across environments |

**Key regression chains (re-verify the earlier phase when the later one lands):**

- **Phase 7 → Phase 6:** adapters inject synthetic `Stopper`s into level
  computation — re-run all level tests **and re-run the generated behavioral tests
  + goldens** (drift alone does not prove behavioral correctness of wrong output).
- **Phase 7 → Phase 3:** adapters change teardown ordering — re-run all ordering
  invariants against an adapter-dependent fixture.
- **Phase 5/6 change → generated behavior:** a parser/analysis change can commit
  *wrong* generated output that still passes drift (drift only proves "matches the
  committed copy"). Always **re-run the Phase 8 golden tests and the Phase 10
  end-to-end behavioral suite**, not just the drift diff.
- **Phases 1–4 → Phase 8:** any runtime-contract change invalidates golden files —
  regenerate and review diffs deliberately.
- **Phase 4 behavioral change → generated apps:** `RunUntilSignal` semantics can
  change without a signature change — re-run the Phase 10 end-to-end suite, not only
  golden diffs.
- **Phase 8/9 → Phase 1 API-surface golden:** after generated symbols exist,
  re-run the **Phase 1** API-surface golden to confirm the framework surface gained
  **no** new exports beyond the runtime-support package (ADR-010), and generated
  code compiles against those frozen surfaces only.
- **Metadata/observability across Phase 8:** re-verify that component identity
  (`ComponentFromContext`) and operation identity (conveyed by the interceptor
  method, ADR-007) and duration-measurability survive into generated output — an
  integration check, so the metadata contract can't silently regress at the
  generation boundary.
- **Phase 8 → Phase 3:** generated code must satisfy Phase 3's behavioral suite —
  run that suite against generated output, not just against hand-written fakes.
- **Anything → Phase 10:** the drift check is the backstop; a diff there means
  either an intended regeneration (review + commit) or accidental drift (fix).

**Open decisions:** none blocking. Most design decisions are recorded in the
canonical docs (PRD, ADR-001…010, Architecture). A small number are plan-level
defaults that fill a genuine gap in those docs rather than restate a documented
decision — each is called out at its point of use so it isn't mistaken for ADR
policy: the overrun log sink (Phase 2, now pinned to a concrete `slog` shape), the
graph-participant panic-recovery policy (Phase 2/3 — the docs define panic
handling only for boundary nodes), `RunUntilSignal`'s default signal set when none
is passed (Phase 4), and the `internal/bridge` package (Phase 1/2) that
Go visibility rules require but no ADR names. The runtime-support-package
sanity-check at Phase 3's generated-shape pin (ADR-010) remains a confirmed-in-flight
default as before.
