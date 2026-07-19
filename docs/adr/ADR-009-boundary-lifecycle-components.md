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
* Execution is best-effort. A failing or panicking boundary component does not prevent
  the pass from proceeding.
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
ordering — front or back — and nothing else. A component's behavior comes entirely from
the lifecycle interfaces it implements, the same interfaces a graph component uses.

Each boundary is kept flat and unordered on purpose. The mechanism grants only a
coarse before-all or after-all pin. Anything that needs an ordering relative to
specific other components has a real dependency relationship and belongs in the
construction graph, where that relationship is expressed once and analyzed like any
other edge. Ordering within a boundary set would recreate a second ordering
mechanism beside the graph.

Best-effort execution follows the shutdown philosophy of ADR-006: a boundary component
cannot fail a pass any more than a graph component can.

## Consequences

### Positive

* "Run before all" and "run after all" become declarative, with no brittle
  dependency edges to maintain as roots change.
* Boundary components reuse the existing lifecycle interfaces and the existing passes;
  there is no separate execution path to reason about.
* The construction graph stays the sole home for genuine ordering relationships.

### Negative

* Boundary components consume the shared deadline of the pass they run in, so a slow
  begin component leaves less time for the rest of that pass. This is documented, not
  mitigated.
* Because execution is best-effort, a boundary component's failure is invisible to the
  caller and observable only through interceptors.
* Boundary sets offer no ordering; a caller that assumes ordering within a set is
  mistaken.

### Accepted Trade-Off

The project accepts a coarse, unordered, best-effort boundary mechanism in exchange
for keeping all genuine ordering in the construction graph and avoiding a second
ordering system.

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

## Non-Goals

Boundary components do not provide:

* Ordering among components in the same boundary set.
* A dependency relationship between boundary components and graph components.
* A new lifecycle phase.
* A guarantee that a boundary component completes or succeeds.

Boundary components exist only to register work that runs before all graph components or after
all graph components, without expressing that position through the construction graph.
