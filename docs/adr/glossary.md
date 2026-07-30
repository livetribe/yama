# Glossary

Shared terms for the PRD, ADRs, Architecture doc, and implementation plan. An
ADR or the Architecture doc uses a term from here rather than defining it
again in prose; a document that needs a new term proposes it here first,
in the same commit that introduces it.

- **Component** — a value created by a statement in the generated injector
  body. The single term for a graph value across every document; do not use
  "node" or "participant."
- **Graph component** — a component reached by walking the injector's
  dependency edges, as opposed to a boundary component.
- **Boundary component** — a component supplied at the call site via
  `WithBeginComponents`/`WithEndComponents` rather than appearing in the
  generated injector body. See ADR-009.
- **Lifecycle-capable component** — a component whose type implements at
  least one of `Starter`, `Quiescer`, `Stopper`.
- **Level** — one entry in the dependency-ordered list Yama computes over
  every component that occupies a level. Components in the same level have
  no ordering dependency between them.
- **Occupies a level** — a component is lifecycle-capable, or is
  dependency-only with a cleanup. A component with neither trait is
  traversed for ordering purposes but occupies no level of its own.
- **Cleanup** — a Google Wire cleanup function returned by a provider. A
  compatibility mechanism, not a lifecycle capability in its own right; see
  ADR-008.
- **Plan** (rejected term) — a description of execution, separate from the
  code that performs it, that an engine would read. Yama's ordered level
  list is not a plan: see ADR-004.
