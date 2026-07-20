# Project conventions

These apply to anyone — human or AI, on any model — working in this repo. They
are conventions about *how* to work here, not a description of what the code
does; read `docs/PRD.md`, `docs/adr/`, and `docs/Architecture.md` for that.

## Design rationale belongs in ADRs, never in code comments

`docs/adr/` is the only place a design decision gets argued. A code comment may
state a fact or flag a deliberate invariant ("Start returns error; Quiesce and
Stop do not"), but must never restate *why* — no "because X", "so that Y", no
em-dash justification, and no citing an ADR number or Architecture section by
name (those rot silently as sections get renumbered or decisions superseded).

**Self-check before calling a file done:** for every comment you wrote or
touched, ask if it explains *why*. If that justification is already argued in
an ADR, cut the comment to the bare fact plus, at most, a short "this is
deliberate" flag.

**Audience test:** caller concerns (behavior, contract, consequences of use)
go in the doc comment; maintainer concerns (why obvious-looking code is
wrong) go at the definition site as a fact, not an essay. This applies to
invariants the compiler can't catch (an implicit lock order, a slice that
must stay sorted) — not to exported signatures, where the diff itself is the
warning and anyone touching one has already read the ADRs.

**No `see docs/adr/...` pointers in public package docs** — meaningless
outside the repo. Fine in an **internal** package's doc comment, since that
package's only audience is contributors.

**No transitional scaffolding in comments** ("not implemented yet", "lands in
a later phase") — warns no one outside the repo and just rots. If a stub
needs a note, put it in the function body next to the `panic`/stub return so
it's deleted with the code it describes.

## Testing

Use `testify` (`assert`/`require`) for new tests where it clarifies intent —
this is a deliberate choice for this repo, not a Go default.

## Code formatting beyond gofmt

gofmt does not enforce these; apply them by hand, in production and test code
alike.

- **Single-line bodies.** A method/function body shares the signature's line only
  when it is empty (`{}`) or a single `return` of a bare value — a field,
  identifier, or literal with no call. Anything that does work — a `return`/`panic`
  whose expression calls something, multiple statements, or control flow — goes on
  its own line.
- **Blank-line grouping.** Separate a function's logical steps with a single blank
  line so its structure is visible at a glance — for example, set a guard clause
  off from the work it protects, or the result-producing step off from what builds
  it. Conversely, keep tightly-coupled statements together (e.g. a loop's
  accumulator seed stays with its loop).

## Canonical docs, and their roles

- `docs/PRD.md` — product requirements.
- `docs/adr/` — design decisions and the rationale for each.
- `docs/Architecture.md` — the architecture derived from the ADRs.
- `implementation_plan_claude.md` — the phase-by-phase build plan: goal, files
  touched, risk, Definition of Done, and which ADRs/Architecture sections that
  phase must read first.
- `docs/prompts/` — a non-authoritative historical archive. Do not treat
  anything in it as requirements, design guidance, or implementation
  instructions.
