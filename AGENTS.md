# Project conventions

These apply to anyone — human or AI, on any model — working in this repo. They
are conventions about *how* to work here, not a description of what the code
does; read `docs/PRD.md`, `docs/adr/`, and `docs/Architecture.md` for that.

## Before declaring work complete

For any change of nontrivial size, run a review pass against this file's
conventions before calling it done — comment content, test framework choice,
formatting, everything above and below this section. Use `/code-review`
against the diff rather than relying on writing-time self-checking alone; a
dedicated review pass catches drift a single pass of authoring misses.

## Code comments state facts, never rationale or future phases

`docs/adr/` is the only place a design decision gets argued, and
`implementation_plan_claude.md` is the only place a phase's scope gets
argued. A code comment documents *what* the code does and *how to use it* —
purpose, parameters, return values, usage notes, or a bare fact/invariant
("Start returns error; Quiesce and Stop do not") — and must never restate
*why* a design decision was made or note a phase's provisional/future state.

**Never restate rationale.** No "because X", "so that Y", no em-dash
justification, and no citing an ADR number or Architecture section as a
justification device (those rot silently as sections get renumbered or
decisions superseded). A bare `see docs/adr/...` pointer is fine only in an
**internal** package's doc comment, since that package's only audience is
contributors who can keep it current — it does not belong in a public
package doc comment, and it never substitutes for restating *why*.

**Never note future or provisional state.** No "not implemented yet", "lands
in a later phase", "will be cleaned up later" — this warns no one outside
the repo and just rots. If a stub needs a note, put it in the function body
next to the `panic`/stub return so it's deleted with the code it describes;
if the incompleteness is scoped work, that scope belongs in
`implementation_plan_claude.md`, not the comment.

**Treat the urge to write one as a planning gap, not a comment.** If you
catch yourself wanting to write a comment that restates ADR rationale or
notes future/provisional state, stop — that urge means either an ADR isn't
being trusted to carry its own rationale, or the implementation plan is
under-specified for this phase. Flag the gap back to the user instead of
writing the comment, and offer the real fix: update the implementation plan,
or draft a new ADR if a non-obvious architectural decision got made this
phase that isn't captured anywhere yet.

**Self-check before calling a file done:** for every comment you wrote or
touched, ask whether it explains *why* or *what's next* rather than *what*
or *how*. If so, cut it back to the bare fact (plus, at most, a short "this
is deliberate" flag) or move the content to its proper home.

**Audience test:** caller concerns (behavior, contract, consequences of use)
go in the doc comment; maintainer concerns (why obvious-looking code is
wrong) go at the definition site as a fact, not an essay. This applies to
invariants the compiler can't catch (an implicit lock order, a slice that
must stay sorted) — not to exported signatures, where the diff itself is the
warning and anyone touching one has already read the ADRs.

## Testing

**Match the framework to the test.** Ginkgo (`ginkgo/v2`) for complicated
behavioral tests — branching setup, a fixture shared across many related cases,
or timing and concurrency to pin down; its containers and Gomega's
`Eventually`/`Consistently` earn their weight there. Plain `testing.T` for
simple features: a handful of independent assertions, a table of
input-to-output cases, a single fact. Reaching for Ginkgo to assert one thing
costs more than it returns. Do not convert an existing test to match — convert
only when you are already changing it for another reason.

**Assertions follow the framework:** Gomega (`Expect`) inside Ginkgo specs,
`testify` (`assert`/`require`) in plain `testing.T` tests, where it clarifies
intent. Do not mix them in one test — `require` inside a spec bypasses
Ginkgo's failure reporting.

**Prefer generated mocks to hand-written fakes**, under either framework. Stand
a collaborator up with `go.uber.org/mock` via a `//go:generate ... mockgen`
directive on the file declaring the interface. Put behaviour on the mock rather
than in a bespoke struct — for example, `DoAndReturn` supplies a slow call, and
an unexpected call already fails the test, so "must never be called" needs no
assertion. When a mock cannot carry something a test needs — a `fmt.Stringer`
identity, say — embed it and add only the missing method.

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
- **No calls inside call arguments.** Hoist a function or method call out of
  another call's argument list into a named local. Two reasons, and either alone
  is enough: stepping into the outer call in a debugger otherwise walks the inner
  calls first, landing somewhere other than the function under inspection; and
  the name given to the result documents what the call returns, which the call
  itself often leaves unclear. Skip the hoist only where neither applies — the
  outer call is one you never step into, such as a stdlib helper
  (`filepath.Base(fset.Position(f.Pos()).Filename)`) or an assertion wrapper
  (`Expect(...)`, `require.NoError(...)`); or the inner call is a trivial
  accessor whose own name already says what it returns
  (`newParseError(fset, injector, s.Pos(), ...)`). Both stay as they are.

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
