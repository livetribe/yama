# ADR-009: Boundary Lifecycle Components

## Status

Accepted

## Context

Some lifecycle work has no natural place in the construction dependency graph. This
work must run before every graph component, or after every graph component. It has no
genuine dependency relationship to any particular graph component.

Expressing "run before all the others" or "run after all the others" through the
construction graph is possible. This method is brittle, though. It requires wiring a
component so that every dependency root depends on it, or so that it depends on every
root. Each time a root is added or removed, that wiring must be revised. The dependency
edges would exist only to force an ordering position. No real dependency would exist.

Yama needs a way to register such components at the start or the end of the lifecycle.
It must do this without hand-maintaining those edges.

## Decision

Yama shall support two boundary registration points that sit outside the construction
DAG: a **begin** boundary and an **end** boundary.

Boundary components are peers of the graph components. A boundary component
participates in the lifecycle through whichever of `Starter`, `Quiescer`, and `Stopper`
it implements. It does this exactly as a graph component does. The boundary in which a
component is registered does not change what the component does. It only fixes the
component's ordering position relative to the graph, as if the component held a
position at the base or the top of the graph's own dependency order:

* A **begin** component behaves as if it held the base position in the graph's
  dependency order. It starts before every graph component. Shutdown is the reverse of
  startup, so a begin component quiesces and stops *after* every graph component. Base
  services such as telemetry belong here. They start first and stop last, so they
  outlive everything that uses them.
* An **end** component behaves as if it held the top position in the graph's dependency
  order. It starts after every graph component. It quiesces and stops *before* every
  graph component. Work that must run as the last step of shutdown belongs here. One
  example is flipping a readiness probe to failing before the graph drains.

Startup order is therefore `begin → graph (dependency order) → end`. Shutdown order is
the exact reverse: `end → graph (reverse order) → begin`. This reverse order applies to
both the quiesce pass and the teardown pass.

The following properties hold:

* Each boundary is a flat, unordered set. There is no ordering guarantee among
  components in the same set.
* A boundary component has no dependency relationship to any *particular* graph
  component. It also has no construction-graph edges. The boundary grants its ordering
  position. The graph does not. A component that has a real dependency on
  specific graph components, or has dependents among them, belongs in the graph, not in
  a boundary.
* Failure semantics are identical to a graph component's. A Start error or panic is
  fail-fast. It surfaces as `ErrStartFailed`, exactly as it would for a graph component
  (ADR-003, ADR-006). A Quiesce or Stop error or panic is recovered. The pass then runs
  to completion. This is exactly how it would work for a graph component. Boundary
  placement changes execution order only. It does not change how a component's failure
  is handled.
* Boundary components run under the same caller context as the pass they join. They
  share its deadline. Yama gives them no budget of their own. A slow begin component
  consumes budget that the rest of the pass would otherwise use.

Boundary components are supplied as runtime objects when the lifecycle value is
constructed, alongside interceptors. They are not a new lifecycle phase. They bracket
the existing three passes and preserve the model established in ADR-003. They are also
not a graph registration API. They never enter the dependency graph that ADR-002 and
ADR-008 make authoritative.

## Rationale

The mechanism exists to express one thing: a component's position at the base or the
top of the whole graph's ordering. A component at the base is one that every graph
component effectively depends on. A component at the top is one that effectively
depends on every graph component. Wiring this position through the construction graph
would mean maintaining dependency edges against every root. Those edges would need
revision whenever the set of roots changes. A boundary registration states the position
directly instead. Because the position matches a base or top position in the graph, the
same ordering rule that governs the graph governs it. Startup runs bases before
dependents. Shutdown runs the exact reverse.

Boundary components are peers, so a boundary registration changes only one thing:
whether a component sits at the base or the top. It changes nothing else, including how
the pass handles the component's failure. A component's behavior comes entirely from
the lifecycle interfaces it implements. These are the same interfaces a graph component
uses. The pass handles a boundary component's failure by the same rules it uses for a
graph component's failure.

Each boundary is kept flat and unordered on purpose. The mechanism grants only a
coarse base or top position. Anything that needs an ordering relative to specific other
components has a real dependency relationship. Such a component belongs in the
construction graph. There, that relationship is expressed once and analyzed like any
other edge. Ordering within a boundary set would recreate a second ordering mechanism
beside the graph.

A single failure model, applied uniformly to graph and boundary components alike, is
simpler than a mechanism with two failure models. A caller reasons about one set of
failure rules, not one rule for components in the graph and a second, weaker rule for
components at a boundary. A caller might want a boundary
component's failure isolated from the pass, treated as optional rather than required.
Such a caller wraps that component so it recovers its own panics and swallows its own
errors. The boundary mechanism itself makes no such accommodation.

## Consequences

### Positive

* "Run before all" and "run after all" become declarative, with no brittle
  dependency edges to maintain as roots change.
* Boundary components reuse the existing lifecycle interfaces and the existing passes.
  There is no separate execution path to reason about.
* The construction graph remains the only place for genuine ordering relationships.
* There is exactly one failure model in the whole framework. A component's failure is
  handled the same way whether it is in the graph or in a boundary. Boundary placement
  needs no separate explanation.

### Negative

* Boundary components consume the shared deadline of the pass they run in. A slow
  begin component therefore leaves less time for the rest of that pass. This is
  documented, not mitigated.
* A boundary component's failure is just as serious as a graph component's failure.
  A failing begin or end `Starter` fails startup (`ErrStartFailed`), just as a failing
  graph `Starter` would. A caller that registers genuinely non-essential work, such as
  a metrics ping or an optional readiness flip, as a boundary component accepts this
  risk. That caller must wrap the component itself if it wants the failure isolated.
* Boundary sets offer no ordering. A caller that assumes ordering within a set is
  mistaken.

### Accepted Trade-Off

The project accepts a coarse, unordered boundary-position mechanism. This mechanism has
no built-in way to mark a boundary component's failure as non-fatal. In exchange, all
genuine ordering stays in the construction graph. The project also keeps exactly one
failure model instead of two. A caller that wants isolated, best-effort behavior for a
specific component gets it by wrapping that component to swallow its own errors and
panics. If that need turns out to be common, a wrapping helper can be added later
without revisiting this decision.

## Rejected Alternatives

### Ordered Boundary Sets

Rejected because ordering relative to specific components is a real dependency
relationship that belongs in the construction graph. An ordered boundary set would
duplicate the graph's ordering role in a weaker, separate mechanism.

### Boundary Components With Graph Dependencies

Rejected because a component with a genuine dependency on, or dependents in, the graph
is a graph component. A boundary component exists precisely for work that has no such
relationship. Expressing one through a boundary would reintroduce the brittle edges the
mechanism avoids.

### A Fourth Lifecycle Phase

Rejected because boundary components do not add a phase. They reuse the existing Start,
Quiesce, and Stop passes and preserve the fixed three-phase model of ADR-003.

### Best-Effort Boundary Failure, Isolated From the Pass

Boundary-component failure could be made best-effort. A failing or panicking begin or
end component would not fail the pass. Its failure would be invisible to the caller.
This alternative is rejected in favor of one uniform failure model. A second,
boundary-only failure policy is exactly the kind of special case the "boundary
components are peers" principle argues against. It also takes control away from a
caller who might actually want a boundary component's failure to matter. A caller that
wants the opposite outcome already has the tool for it. Such a caller wants a boundary
component whose failure is genuinely inconsequential. That caller can wrap the
component so it recovers its own panics and swallows its own errors before Yama ever
sees them.

## Non-Goals

Boundary components do not provide:

* Ordering among components in the same boundary set.
* A dependency relationship between boundary components and graph components.
* A new lifecycle phase.
* A distinct failure-handling policy from graph components.

Boundary components exist only to register work as if it sat at the base or the top of
the graph's own dependency order. A begin component starts before all graph components
and tears down after them. An end component starts after all graph components and
tears down before them. Neither actually joins the construction graph to hold that
position.
