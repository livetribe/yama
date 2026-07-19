# ADR-009: Boundary Lifecycle Components

## Status

Accepted

## Context

Some lifecycle work has no natural place in the construction dependency graph. It
must run before every graph component, or after every graph component, and it has no genuine
dependency relationship to any particular graph component.

Expressing "run before all the others" or "run after all the others" through the
construction graph is possible but brittle. It requires wiring a component so that every
dependency root depends on it, or so that it depends on every root, and revising
that wiring whenever a root is added or removed. The dependency edges would exist
only to force an ordering position, not because a real dependency exists.

Yama needs a way to register such components at the extremes of the lifecycle without
hand-maintaining those edges.

## Decision

Yama shall support two boundary registration points that sit outside the
construction DAG: a **begin** boundary and an **end** boundary.

Boundary components are peers of the graph components. A boundary component participates in the
lifecycle through whichever of `Starter`, `Quiescer`, and `Stopper` it implements,
exactly as a graph component does. The boundary a component is registered in does not change
what it does; it controls only its execution order relative to the graph:

* A **begin** component runs before all graph components in each pass it participates in.
* An **end** component runs after all graph components in each pass it participates in.

The following properties hold:

* Each boundary is a flat, unordered set. There is no ordering guarantee among components
  in the same set.
* A boundary component has no dependency relationship to any graph component. A component that has a
  real dependency on, or dependents in, the graph belongs in the graph, not in a
  boundary.
* Failure semantics are identical to a graph component's. A Start error or panic is
  fail-fast and surfaces as `ErrStartFailed`, exactly as it would for a graph component
  (ADR-003, ADR-006). A Quiesce or Stop error or panic is recovered and the pass runs to
  completion, exactly as it would for a graph component. Boundary placement changes
  execution order only; it does not change how a component's failure is handled.
* Boundary components run under the same caller context as the pass they join and share
  its deadline; Yama gives them no budget of their own. A slow begin component consumes
  budget the rest of the pass would otherwise use.

Boundary components are supplied as runtime objects when the lifecycle value is
constructed, alongside interceptors. They are not a new lifecycle phase — they
bracket the existing three passes and preserve the model established in ADR-003 —
and they are not a graph registration API, since they never enter the dependency
graph that ADR-002 and ADR-008 make authoritative.

## Rationale

The mechanism exists to express one thing: this component runs before everything, or
after everything. Wiring that through the construction graph would mean maintaining
dependency edges that encode a position rather than a real dependency, revised
whenever the set of roots changes. A boundary registration states the position
directly.

Because boundary components are peers, a boundary registration changes only gross
ordering — front or back — and nothing else, including how failure is handled. A
component's behavior comes entirely from the lifecycle interfaces it implements, the
same interfaces a graph component uses, and its failure is handled by the same rules a
graph component's is.

Each boundary is kept flat and unordered on purpose. The mechanism grants only a
coarse before-all or after-all pin. Anything that needs an ordering relative to
specific other components has a real dependency relationship and belongs in the
construction graph, where that relationship is expressed once and analyzed like any
other edge. Ordering within a boundary set would recreate a second ordering
mechanism beside the graph.

A single failure model, applied uniformly to graph and boundary components alike, is
simpler than a mechanism with two: a caller reasons about one set of failure rules, not
one for components in the graph and a second, weaker one for components at its edges. A
caller that wants a boundary component's failure isolated from the pass — treated as
optional rather than required — wraps that component so it recovers its own panics and
swallows its own errors; the boundary mechanism itself makes no such accommodation.

## Consequences

### Positive

* "Run before all" and "run after all" become declarative, with no brittle
  dependency edges to maintain as roots change.
* Boundary components reuse the existing lifecycle interfaces and the existing passes;
  there is no separate execution path to reason about.
* The construction graph stays the sole home for genuine ordering relationships.
* There is exactly one failure model in the whole framework. A component's failure is
  handled the same way whether it sits in the graph or at a boundary; nothing about
  boundary placement needs a separate explanation.

### Negative

* Boundary components consume the shared deadline of the pass they run in, so a slow
  begin component leaves less time for the rest of that pass. This is documented, not
  mitigated.
* A boundary component's failure carries the same weight as a graph component's: a
  failing begin or end `Starter` fails startup (`ErrStartFailed`) just as a failing
  graph `Starter` would. A caller registering genuinely non-essential work (a metrics
  ping, an optional readiness flip) as a boundary component takes on that risk and must
  wrap the component itself if it wants the failure isolated.
* Boundary sets offer no ordering; a caller that assumes ordering within a set is
  mistaken.

### Accepted Trade-Off

The project accepts a coarse, unordered boundary-position mechanism, with no built-in
way to mark a boundary component's failure as non-fatal, in exchange for keeping all
genuine ordering in the construction graph and having exactly one failure model instead
of two. A caller that wants isolated, best-effort behavior for a specific component
gets it by wrapping that component to swallow its own errors and panics; if that need
turns out to be common, a wrapping helper can be added later without revisiting this
decision.

## Rejected Alternatives

### Ordered Boundary Sets

Rejected because ordering relative to specific components is a real dependency
relationship that belongs in the construction graph. An ordered boundary set would
duplicate the graph's ordering role in a weaker, separate mechanism.

### Boundary Components With Graph Dependencies

Rejected because a component with a genuine dependency on, or dependents in, the graph is
a graph component. A boundary component exists precisely for work that has no such
relationship; expressing one through a boundary would reintroduce the brittle edges
the mechanism avoids.

### A Fourth Lifecycle Phase

Rejected because boundary components do not add a phase. They reuse the existing Start,
Quiesce, and Stop passes and preserve the fixed three-phase model of ADR-003.

### Best-Effort Boundary Failure, Isolated From the Pass

An earlier version of this ADR made boundary-component failure best-effort: a failing
or panicking begin or end component would not fail the pass, and its failure would be
invisible to the caller. Rejected in favor of one uniform failure model. A second,
boundary-only failure policy is exactly the kind of special case the "boundary
components are peers" principle argues against, and it takes away control from the
caller who might actually want a boundary component's failure to matter. A caller that
wants the opposite — a boundary component whose failure is genuinely inconsequential —
already has the tool for it: wrap the component so it recovers its own panics and
swallows its own errors before Yama ever sees them.

## Non-Goals

Boundary components do not provide:

* Ordering among components in the same boundary set.
* A dependency relationship between boundary components and graph components.
* A new lifecycle phase.
* A distinct failure-handling policy from graph components.

Boundary components exist only to register work that runs before all graph components or after
all graph components, without expressing that position through the construction graph.
