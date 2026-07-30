# Documentation Churn Post-Mortem: Phases 0–8

Covers `docs/PRD.md`, `docs/adr/`, `docs/Architecture.md`, and
`implementation_plan_claude.md` from `bed2618` (2026-06-18) through `fab55e5`
(2026-07-28) — 51 commits, 27 of which touched a planning document.

## 1. Overall pattern summary

Churn was heavily front-loaded and almost entirely doc-internal: 19 of the 27
doc-touching commits changed no source file at all, and nearly all of those
landed between 7/13 and 7/19, before Phase 1 code (`8275903`) was merged. The
documents moved as a fused block — a single naming or semantics change typically
rewrote the Architecture doc, three or four ADRs, and the implementation plan in
one commit (`bff6f85` touched twelve files). The tail of the period looks
different: from 7/20 onward doc edits ride alongside code and are triggered by
it. Phase 8 is the extreme case and the clearest signal in the whole range — its
only commit, `fab55e5`, changed 891 lines across thirteen documents and not one
line of executable code (its single source edit is 19/16 lines of *comment* in
the pinned exemplar). A phase budgeted for the emitter produced documentation
instead. ADR-007 (12 revisions), ADR-003 (9), and the Architecture doc (~20) were
the churn engines; ADR-001, ADR-002, and ADR-004 were comparatively stable.

## 2. Root cause breakdown

**Premature decision — the single largest contributor.** Nine ADRs were written
on 6/18 in one sitting, before any v2 code existed. Four weeks later, `08a3bf3`
deleted three of them outright (ADR-003 start-drain-stop, ADR-005 generated typed
configuration, ADR-009 wire generation internals), renumbered four survivors, and
rewrote the PRD and Architecture doc around the replacement model. That is a
third of the accepted decision set discarded before the first line of
implementation. The 7/16–7/19 run of naming and modeling commits (`8d9821f`,
`8174542`, `d9f6570`, `bb1f3ad`, `bff6f85`) is the same phenomenon at smaller
scale: decisions recorded as "Accepted" that were in fact still being made, so
each refinement cost an edit to three or four documents instead of a
conversation.

**Genuine mid-implementation learning — a significant minority, and the
healthiest churn here.** Once code existed, several ADR revisions were forced by
facts only implementation could surface: `6d96a8a` (cancelling siblings in a
failing level creates a partial-work state with no owner), `9135845` (a bare
`func()` cleanup carries no context and no identity, so it cannot pass through an
interceptor chain), `e897b02` (Wire writes `wire_gen.go` beside the sources with
no redirect and its generator is `internal/`, so the file must be transient), and
`5ce60fc` (a cleanup-only value occupies a level while carrying no capability).
None were knowable from the whiteboard. They account for most of the post-7/20
churn outside the final reconciliation.

**Misfiled content — real but confined to one correction.** `e6cd81e` trimmed PRD
sections 8–12 (lifecycle, interceptor, error, code-generation, and DAG-analysis
models), roughly 170 lines of design detail duplicating the ADRs. The PRD had
been written as a design document wearing a requirements title. After that single
excision it was stable — touched only four more times in six weeks, the correct
rate for a requirements doc.

**Scope creep — concentrated in the Architecture doc and the implementation
plan.** The Architecture doc accreted implementation-level detail it could not
keep current: `5ce60fc` records that `cleanupAdapter`, "referenced across four
documents," was never an identifier in the tree, and `fab55e5` rewrote 378 of its
lines. The implementation plan drifted the same way — it restated public API
names, so purely cosmetic renames (`97f29df`, `bb1f3ad`, `d9f6570`) dragged it
along for no planning reason.

**Process violation — one systemic instance, not the one that was expected.** The
immutability convention was never broken, because the project deliberately does
not have one: `docs/adr/README.md` states that ADRs are rewritten in place
pre-1.0. But that rule was written on 7/28, in `fab55e5` — after roughly forty
in-place ADR edits had already been made under it implicitly. The substantive
violation is different and worse: over Phases 2–7, design decisions landed in
code with no ADR recording them at all. ADR-011 was written retroactively to
capture the constructor-stub design, and "where the disagreement was fact rather
than design, the code won." The debt did not stay in the phases that created it —
it was paid in full out of Phase 8's budget, which is why Phase 8 shipped no
emitter. Deferred doc maintenance from six phases became one later phase's entire
output.

## 3. What could have been scoped or granularized differently

**Do not mark an ADR "Accepted" before it has survived contact with a prototype.**
(Premature decision.) The 6/18 batch of nine was an act of speculative
completeness; a third of it did not survive. A `Proposed`/`Accepted` distinction
actually used, with acceptance gated on a spike, would have made `08a3bf3` a
normal promotion instead of a mass deletion.

**Keep vocabulary out of the ADRs' prose and in one glossary.** (Scope creep.)
`bff6f85` rewrote twelve files to unify "node"/"participant" into "component."
Every document restated the same terminology independently, so a terminology
decision cost a repo-wide sweep rather than a one-file edit.

**Keep the Architecture doc at the level of shapes, not identifiers.**
(Scope creep.) Naming concrete types and helpers there guaranteed rot the moment
implementation diverged — `cleanupAdapter` was cited in four documents and never
existed in the tree. Identifiers belong in the pinned exemplar
(`internal/generator/sandbox/lifecycle_gen.go`), which the compiler keeps honest.

**Make "does this phase need an ADR?" part of the phase's Definition of Done.**
(Process violation.) Phases 2–7 shipped design decisions with no ADR; ADR-011 was
reconstructed a month later from the code. A per-phase check at merge time would
have converted one 891-line reconciliation into six small, in-context commits —
and left Phase 8 free to build the emitter it was scoped for.

## 4. Illustrative examples

**`08a3bf3` — "Refine lifecycle architecture ADRs" (premature decision).** One
month after the initial nine ADRs were written, this commit deleted three,
renumbered four, and rewrote 1,265 lines across the PRD and Architecture doc. No
implementation had begun. The original set was written before the design
question — parse Wire's output vs. reimplement its internals — had been settled.

**`e6cd81e` — PRD sections 8–12 trimmed (misfiled content).** The PRD carried the
lifecycle, interceptor, error, code-generation, and DAG-analysis models in full.
Removing them left the PRD stating requirements and the ADRs owning decisions,
and the PRD's revision rate dropped sharply afterward.

**`9135845` — "Demote Wire cleanup functions to a compatibility shim" (genuine
learning).** Cleanups had been documented as the primary teardown mechanism, each
becoming a synthetic `Stopper`. Writing the runtime showed a bare `func()` has no
context to observe a deadline and no identity to attribute an observation to, so
it cannot be a component. The design was demoted to a compatibility path.

**`5ce60fc` — "Separate level occupancy from lifecycle capability" (genuine
learning, plus rot).** The docs claimed only lifecycle-capable components take
part in execution; the runtime had never worked that way. The same commit
corrected an identifier referenced in four documents that never existed.

**`fab55e5` — "Reconcile design docs with the implementation" (process
violation).** The docs described generated level structs with ordering methods;
what shipped was a builder call chain into `rt`. ADR-004 and ADR-010 were
rewritten, ADR-011 added retroactively, and the in-place-rewrite convention was
codified after the fact. It is also the whole of Phase 8: thirteen documents
changed, no executable code, and the emitter the phase existed to build not
started. The cost of Phases 2–7 running without a doc-update gate was billed to
the next phase.
