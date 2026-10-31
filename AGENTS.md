# AGENTS.md

## Required reads

Before making any code or test edits, read `CODING_STANDARDS.md`. It holds the
rules that apply to every edit. If this file and `CODING_STANDARDS.md` differ,
follow the stricter rule. "Code comments state facts, never rationale or future
phases" below overrides the general comment rule in `CODING_STANDARDS.md`.

This file states how to work here. For what the code does, read `docs/PRD.md`,
`docs/adr/`, and `docs/Architecture.md`.

## Skills

Use these skills from `.claude/skills/` when the task matches:

| Skill | Use it when |
|---|---|
| `go-testing` | Writing or changing a test |
| `git-commit-message` | Writing a commit message |
| `asd-ste100` | Writing or checking a code comment or a commit body |
| `adr-writer` | Writing an ADR, or finishing one drafted by hand |
| `architecture-doc-check` | Changing `docs/Architecture.md` |

`.claude/workflows/` holds the `adr-validate` and `adr-validate-lite` workflows.
They run a final adversarial review of an ADR.

## Project overview

- Language: Go
- Module: `l7e.io/yama`
- Primary goals: Generate code that starts, quiesces, and stops a Google Wire
  dependency graph in dependency order, and supply the runtime that the code
  uses
- Preferred Go version: `1.26.x` (see `go.mod`)
- OS support: macOS, Linux, Windows

## Commands

- Build: `make build`
- Format: `make fmt`
- Format check: `make check-fmt`
- Tests with the race detector: `make test`
- Coverage: `make coverage`
- Lint: `make lint`
- Vulnerability check: `make vuln`
- Code generation: `make generate`
- Module tidy: `make tidy`
- Clean: `make clean`
- Gate target: `make check` (`test`, `lint`, and `check-fmt`)
- CI target: `make ci` (`check` and `coverage`)

For each change, run `make fmt` and `make check`.

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

**Never note a phase's state.** No "lands in a later phase". If the
incompleteness is scoped work, that scope belongs in
`implementation_plan_claude.md`, not the comment.

**A comment that fails is a planning gap.** If a comment fails the audience
test in `CODING_STANDARDS.md` by explaining *why* or *what's next*, that is a
planning gap, not a wording problem: flag it back to the user instead of
writing it, and fix the real gap — update the implementation plan, or draft a
new ADR if this phase settled a non-obvious decision nothing captures yet.

## Testing

**A new public package needs a golden file.** `TestAPISurface`
(`api_surface_test.go`) walks `./...` and snapshots every non-`internal`,
non-`main` package's exported symbols into
`testdata/api_surface/<import-path>.golden`. Adding a package's first
exported symbol makes the test discover it automatically and fail
immediately, since no golden exists for it yet — that failure is expected,
not a bug. Generate the golden deliberately with
`go test -run TestAPISurface -update .` and review the diff before
committing; it is the record of what that package now promises to expose.

**Yama examples of the `slog` rule in the `go-testing` skill.** Production code
in this module logs recovered panics and skipped steps via `slog`'s
package-level default. For a capturing handler, see the `captureSlog` helper in
`rt/internal/exec`. For a discard handler, see the `BeforeSuite` in
`rt/rt_suite_test.go`.

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
