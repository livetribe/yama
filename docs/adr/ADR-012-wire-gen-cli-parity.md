# ADR-012: The Yama Command Mirrors `wire gen`

## Status

Proposed

## Context

Yama's command line is meant to behave like Google Wire's `gen` command.
`cmd/yama/main.go` and `internal/generator/wire.go` already say this much, in
their own doc comments. Those comments state that the flags mirror `wire gen`'s
on purpose. No document
says how far that mirroring reaches once a question isn't about a flag, what
deciding it settles, or what it costs. This ADR states that decision, and the
rest of this Context section shows the evidence that it was never written down.

One `go:generate` directive invokes Yama for generation. Yama then runs Google
Wire internally (ADR-008). That directive stands where a line naming Wire's own
command stands, whether it replaces one or is added beside one, so a project
usually has such a line already to compare against. Wire writes this directive
into every file it generates:

```
//go:generate go run -mod=mod github.com/google/wire/cmd/wire
```

`gen` is Wire's default subcommand, so a bare `wire` invocation and an explicit
`wire gen` do the same thing. This document names the behaviour as `wire gen` for
precision, not because a directive is usually spelled that way.

Yama's flags mirror that subcommand's flags exactly: `-header_file`,
`-output_file_prefix`, and `-tags`. Yama's positional argument is Wire's own
package-pattern list, and it defaults to `.`.

The project matched the flags on purpose. It decided every other question case
by case, in whichever file needed an answer. This left a contradiction that
nobody noticed. `internal/generator/multi.go`'s `GenerateAll` skips a package
with no injector in silence, matching Wire. Its doc comment states this. A
test asserts it. The implementation plan's Phase 9 Definition of Done once
required a clear error for the same case instead. This ADR's Decision settles
which of the two the code keeps. The plan has since been corrected to match
it. Both were written in good faith. Nothing existed to settle the
disagreement between them before this ADR.

More such questions remain open. Each one invites the same argument. They
include: exit codes; what the command prints on success; which packages
produce output and which the command passes over; what happens when a pattern
resolves to nothing; what an error looks like. Answering each one on taste
builds a second
command-line vocabulary. A Wire user must learn that vocabulary. The tool
assumes they already know Wire.

## Decision

The Yama command's observable behaviour matches `wire gen`'s. The one
substitution is that Yama commits `lifecycle_gen.go`, not `wire_gen.go`.

Another ADR may settle a question about the command's observable behaviour.
Where none does, the answer is whatever `wire gen` does — established by running
it, not by argument.

"Observable behaviour" is the surface a caller or a CI script can see: flag
names, defaults, and meanings; the package-pattern argument and its default; exit
codes; what reaches stdout and stderr; which packages produce output and which
are passed over; the shape of a diagnostic.

The table below states `wire gen`'s own measured behaviour, in Wire's own terms:
`wire_gen.go`, Wire's own progress line. This behaviour was established by
running `wire gen` against a fixture module holding one package with an
injector and two without. Yama's behaviour is this same behaviour with the one
substitution already
stated: `lifecycle_gen.go` in place of `wire_gen.go`, throughout.

| Scenario | `wire gen`'s behaviour |
| --- | --- |
| Named package, has an injector | exit 0; writes `wire_gen.go`; prints one progress line naming it |
| Named package, no injector | exit 0; writes nothing; prints nothing |
| Wildcard matching a mix of packages | exit 0; writes `wire_gen.go` and prints a line for each package with an injector; passes over the rest in silence |
| Several named packages, none with an injector | exit 0; writes nothing; prints nothing |
| A pattern that matches no package at all (e.g. a mistyped path) | exit 1; prints a diagnostic naming what it could not resolve |

Yama necessarily does more than Wire in two places. Both follow from the
substitution above. Neither is an exception to it.

* **Wire's output is transient.** Yama runs Wire, parses `wire_gen.go`, and
  removes it. If a file of that name already existed, Yama preserves it and
  restores it afterward (ADR-008). Wire leaves its own output in place, because
  that output is Wire's artifact. Yama's artifact is `lifecycle_gen.go`.
* **Progress output names Yama's artifact.** Wire prints one line for each file
  it writes. Yama prints one line for each `lifecycle_gen.go` that it writes.
  It does not echo Wire's line for the transient file. That line would
  announce a file that will not exist once the command returns.

This ADR governs `gen`-parity only. `gen` is the only Wire subcommand that
Yama's CLI implements today. Yama's CLI takes its patterns as bare arguments,
with no subcommand dispatch, exactly as a bare `wire` invocation does. `go tool wire
help` lists five other subcommands: `check`, `diff`, `show`, `commands`, and
`flags`. This ADR takes no position on whether Yama should ever mirror them. No
document does.

## Rationale

### The drop-in swap is the adoption story

An application that adopts Yama already uses Google Wire (ADR-002). It
therefore already has a working `go:generate` line naming Wire's command. Parity keeps the
migration to that one line and nothing else. Each behavioural difference is one
more thing the migration has to explain.

### It converts a question of taste into a measurement

Take the question: what should the command do when the target package has no
injector? Two answers are defensible. The project asserted both at once, in
different documents, for as long as nobody compared them. Under this decision
the question becomes "run `wire gen` and look": cheap, reproducible, and
settled in one command. The contradiction behind this ADR survived unnoticed
through two phases. This check resolved it in minutes.

### Wire's answers are load-bearing, not incidental

The silent skip is what makes `yama ./...` usable at all. In any real
repository, the overwhelming majority of packages hold no injector. A tool
that failed on each of them could run only one package at a time. This is not
an accident of Wire's implementation. It is the behaviour that the pattern
argument requires.

### Deference is cheaper than divergence, and reversible

Where Yama should genuinely differ from Wire, a later ADR can argue that case
against this stated default. Without a default, each question is argued from
nothing. Its answer ends up recorded in whatever file the person answering it
happened to be working in. Sometimes it is not recorded anywhere at all.

## Consequences

### Positive

* An application that wants Yama to own generation points its existing
  `go:generate` directive at Yama. Only the named tool changes, because the flags
  and the package argument are the same. An application that keeps its own
  directive naming Wire's command adds Yama's beside it, and writes the same
  flags there.
* A command-line question now has a decision procedure instead of a discussion,
  and that procedure gives the same answer to everyone who runs it.
* A Definition of Done can state measured behaviour instead of assumed
  behaviour. The implementation plan already does this for the no-injector
  case.

### Negative

* Yama inherits Wire's command-line judgements, including ones Yama would not
  make on its own. A named package with no injector exits 0 in silence. This
  hides a mistyped path that still names a real package, exactly as it hides
  an irrelevant one. It is a different case from the table's mistyped-path
  example above, which names no package at all and exits 1 loudly.
* Yama now depends on Wire's observable behaviour, so a Wire version bump can
  shift it. ADR-008 already accepts a coupling to Wire's generated output.
  This decision extends that coupling to Wire's command-line surface. Phase 10's
  drift check guards against this, and records the Wire version tested.
* A parity claim is only as good as the fixture that measured it, so "matches
  Wire" is a statement about scenarios someone actually ran.

### Accepted Trade-Off

The project accepts two costs: command-line judgements it did not make, and a
dependency on Wire's observable behaviour that only tests can pin down. In
exchange, it gets a genuine drop-in replacement and a mechanical answer to every
question about the command.

## Rejected Alternatives

### Design the command independently

Rejected. It makes every question an argument. The accumulated answers become
a second command-line vocabulary. A Wire user must learn that vocabulary. The
tool assumes they already know Wire. It also gives up the one-line migration.

### Match Wire's flags, decide behaviour separately

Rejected. It has two rules, not one. One rule governs flags: match Wire. A
different rule governs behaviour: decide it separately. No principle says which
rule covers a new question. Nobody chose this as a deliberate position. It is
what happens by default without one. That default is how a doc comment and a
Definition of Done ended up stating opposite answers for the same question,
with no stated standard to say either one was wrong at the time.

## Non-Goals

This decision does not do these things:

* It does not govern what Yama generates, or how Yama derives ordering (ADR-008,
  ADR-010, ADR-011). It governs only the command's observable behaviour.
* It does not govern the runtime public API (ADR-007), which mirrors nothing in
  Wire.
* It does not commit Yama to Wire's internal behaviour, source layout, or output
  formatting. It commits Yama only to what Wire's command line does.
* It does not decide whether Yama should someday mirror Wire's other
  subcommands (see Decision).
