---
name: architecture-doc-check
description: Check a docs/Architecture.md change for concrete identifiers (function, type, or variable names) that don't exist in the current codebase. Use before treating an Architecture.md edit as done, and whenever docs/Architecture.md appears in a diff being reviewed.
---

# Architecture doc identifier check

`docs/Architecture.md` describes the shape of the design — components,
levels, capabilities, passes — not the specific identifiers that implement
it. Naming a concrete function, type, method, or variable there creates a
second place that name has to stay correct; the file drifts silently the
moment that identifier is renamed or removed, because nothing rebuilds or
tests the doc. Commit `5ce60fc` ("Separate level occupancy from lifecycle
capability") documents a real instance of this: `cleanupAdapter` was
referenced across four documents and was never an identifier anywhere in the
tree.

## What to do

1. Get the diff of `docs/Architecture.md` under review (`git diff` against
   the target branch, or the full file if no baseline diff is available).
2. Pull out tokens that look like Go identifiers rather than prose: anything
   backtick-quoted or otherwise formatted as code, and any
   `CamelCase`/`camelCase` word that reads as a symbol name rather than a
   concept already in `docs/adr/glossary.md`. Capability interface names
   (`Starter`, `Quiescer`, `Stopper`) and public API surface named in
   ADR-007 are expected and not findings — the check is for
   implementation-internal identifiers (unexported helpers, concrete generated
   names, package-internal type names).
3. For each candidate identifier, `grep -rn` the codebase (excluding
   `docs/`) for it. Go through `internal/`, `rt/`, the module root, and any
   generated exemplar (`internal/generator/sandbox/lifecycle_gen.go`).
4. Report identifiers that don't grep-match anything, plus identifiers that
   match but describe a shape the surrounding prose doesn't actually need
   (i.e. the doc would be simpler and equally correct stating the shape
   without naming the symbol).
5. For each finding, recommend either deleting the identifier from the prose
   in favor of a shape-level description, or — if the identifier is genuinely
   load-bearing for a reader (e.g. it's the one public entry point a caller
   uses) — leave it, since this check is about *internal* identifiers rot,
   not all code references.

Report findings as a short list: file, line, the identifier, and whether it
was found in the tree. Do not silently fix the doc — surface findings so the
author decides what belongs at shape-level versus what's worth pinning.
