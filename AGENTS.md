# Project conventions

These apply to anyone — human or AI, on any model — working in this repo. They
are conventions about *how* to work here, not a description of what the code
does; read `docs/PRD.md`, `docs/adr/`, and `docs/Architecture.md` for that.

## Go style baseline

This repo's Go style baseline is the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
Follow it for anything this file doesn't cover — method and function ordering,
error handling and naming, receiver types, struct literals, and so on. Where a
rule below conflicts with Uber's guide, this file wins; treat everything below
as this project's stated exceptions and additions, not a full restatement.

`.golangci.yml` enforces the subset of the guide a linter can check
mechanically (`unparam`, `unconvert`, `ineffassign`, `gosec`, `gocritic`'s
`opinionated`/`style` tags, and similar). The rest — ordering, naming,
grouping — isn't linter-checkable and depends on the `/code-review` pass
below.

## Before declaring work complete

For any change of nontrivial size, run a review pass against this file's
conventions before calling it done — comment content, test framework choice,
formatting, adherence to the Go style baseline above, everything above and
below this section. Use `/code-review` against the diff rather than relying
on writing-time self-checking alone; a dedicated review pass catches drift a
single pass of authoring misses.

A change that touches code is not proposed as done until `make check` passes,
run in the actual working tree, not a scratch copy or worktree.

## Code comments
All code comments must follow ASD-STE100 (Simplified Technical English):
short sentences, one instruction per sentence, active voice, approved
vocabulary, no jargon or idioms. Use the `asd-ste100` skill to write or check
comments against this standard — do not rely on general-purpose simplifying.

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

**Self-check before calling a file done:** for every comment you wrote or
touched, apply the audience test — caller concerns (behavior, contract,
consequences of use) belong in the doc comment; maintainer concerns (why
obvious-looking code is wrong) belong at the definition site as a bare fact,
not an essay. That covers invariants the compiler can't catch (an implicit
lock order, a slice that must stay sorted), not exported signatures, where
the diff itself is the warning and anyone touching one has already read the
ADRs. If a comment fails the test by explaining *why* or *what's next*, that
is a planning gap, not a wording problem: flag it back to the user instead
of writing it, and fix the real gap — update the implementation plan, or
draft a new ADR if this phase settled a non-obvious decision nothing
captures yet.

## Where the package doc comment lives

Every non-`main` package has a package comment, in exactly one file, starting
with the package name: `// Package graph ...`.

A package with more than one source file puts that comment in `doc.go`. That
file holds the package comment and nothing else, except `//go:generate`
directives. A package with exactly one source file puts the comment at the top
of that file; the filename does not matter, and a file named after the package
does not exempt a multi-file package from `doc.go`.

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

**A new public package needs a golden file.** `TestAPISurface`
(`api_surface_test.go`) walks `./...` and snapshots every non-`internal`,
non-`main` package's exported symbols into
`testdata/api_surface/<import-path>.golden`. Adding a package's first
exported symbol makes the test discover it automatically and fail
immediately, since no golden exists for it yet — that failure is expected,
not a bug. Generate the golden deliberately with
`go test -run TestAPISurface -update .` and review the diff before
committing; it is the record of what that package now promises to expose.

**A suite that doesn't assert on log content should silence slog, not print
it.** Production code in this module logs recovered panics and skipped steps
via `slog`'s package-level default. A spec that needs to check what got
logged installs a capturing handler for the duration (see
`rt/internal/exec`'s `captureSlog` helper) and restores the previous default
after. A suite that exercises those logging paths only incidentally — to
prove recovery behavior, not to inspect the log — should instead silence the
default for the whole run (a `BeforeSuite` swapping in a discard handler; see
`rt/rt_suite_test.go`) rather than let it print to stderr on every green
pass.

## Code formatting beyond gofmt

gofmt does not enforce these; apply them by hand, in production and test code
alike.

- **Method ordering.** This intentionally deviates from Uber's guide, which
  sorts by call order and allows an unexported helper to sit next to its one
  caller; this project instead partitions every type's methods into exported
  before unexported, favoring breadth-first reading of a type's public
  surface over call-order locality. Group methods by receiver, with a
  `NewXYZ` constructor immediately after its type definition. Place every
  exported method before any unexported one, so a reader can take in the
  type's public surface breadth-first without an unexported implementation
  detail interrupting it; order the exported methods themselves in rough
  call order (e.g., `Start` before `Stop`). If the type implements more than
  one interface, group its exported methods by the interface they satisfy
  rather than interleaving them. Unexported helpers follow, each ordered
  near the exported method it supports. Unattached utility functions go
  last in the file.
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

## A phase that makes a design decision writes the ADR before the phase is done

`implementation_plan_claude.md` plans a phase's scope; it does not record the
decisions made while executing it. If a phase's work settles a question the
plan left open — a naming choice, an ordering rule, a rejected alternative —
write or update the ADR in the same phase, not later. A decision that reaches
`master` only in code and a commit message is undocumented, regardless of
how clearly the commit message explains it: nothing keeps that reasoning
findable once the commit scrolls out of recent `git log` output. Treat "does
this phase's work need an ADR?" as part of that phase's Definition of Done,
alongside its stated output checks.

## Canonical docs, and their roles

- `docs/PRD.md` — product requirements.
- `docs/adr/` — design decisions and the rationale for each.
- `docs/adr/glossary.md` — shared terminology; ADR and Architecture prose uses
  these terms rather than redefining them.
- `docs/Architecture.md` — the architecture derived from the ADRs, at the
  level of shapes and roles, not concrete identifiers. Run the
  `architecture-doc-check` skill before treating an edit to this file as
  done.
- `docs/guide.md` — the behavior reference for applications that use Yama:
  the lifecycle model, ordering, errors, interceptors, the context, and the
  command. States behavior only; the ADRs keep the rationale.
- `implementation_plan_claude.md` — the phase-by-phase build plan: goal, files
  touched, risk, Definition of Done, and which ADRs/Architecture sections that
  phase must read first.
- `docs/prompts/` — a non-authoritative historical archive. Do not treat
  anything in it as requirements, design guidance, or implementation
  instructions.
