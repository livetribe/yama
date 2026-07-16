**1\. Coverage Gaps**

Most lifecycle ordering requirements are mapped. Gaps:

*   PRD §10 says a start that exceeds its context deadline surfaces as ErrStartFailed (\[docs/PRD.md (line 480)\](/Users/adc/dev/yama/docs/PRD.md:480)). The plan only has a generic overrun wrapper and an edge case for “caller context already canceled at Start” (\[implementation\_plan\_claude.md (line 289)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:289)) without specifying the expected ErrStartFailed result or cleanup behavior.

*   PRD §13 requires observability of “duration measurements” (\[docs/PRD.md (line 528)\](/Users/adc/dev/yama/docs/PRD.md:528)). The plan tests generic interceptor observation, but no phase explicitly verifies duration can be measured or that wrappers preserve start/end visibility.

*   ADR metadata coverage is incomplete: ADR-005 says lifecycle metadata examples include component identity and operation identity (\[docs/adr/ADR-005-lifecycle-interceptors.md (line 241)\](/Users/adc/dev/yama/docs/adr/ADR-005-lifecycle-interceptors.md:241)). The plan only covers component identity via ComponentFromContext.


**2\. Silent Assumptions**

*   The plan adds ComponentFromContext to the stable runtime API: “ErrStartFailed, ComponentFromContext, the boundary options…” (\[implementation\_plan\_claude.md (line 15)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:15)). ADR-007’s public helper list does not include it (\[docs/adr/ADR-007-public-api.md (line 161)\](/Users/adc/dev/yama/docs/adr/ADR-007-public-api.md:161)). This is a public API expansion without being flagged as one.

*   The plan narrows metadata to component-only: “Accessor exposes component metadata only” (\[implementation\_plan\_claude.md (line 152)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:152)). That papers over ADR-005/ADR-006’s operation-identity diagnostic requirement.

*   The plan assumes a handwritten runtime execution engine is acceptable: “Runtime execution engine: levels, fail-fast, shutdown, boundaries” and run.go / level.go with “level runner, started-flag tracking, cleanup path” (\[implementation\_plan\_claude.md (line 220)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:220)). ADR-004 explicitly rejects runtime orchestration engines (\[docs/adr/ADR-004-generated-execution-plans.md (line 58)\](/Users/adc/dev/yama/docs/adr/ADR-004-generated-execution-plans.md:58)).

*   The helper APIs are being specified without a flagged decision. Example: RunInBackground assumes “callback/channel” and a “sink” behavior (\[implementation\_plan\_claude.md (line 322)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:322)), while ADR-007 leaves exact signatures undefined.


**3\. Sequencing Errors**

*   Phase 6 says its dependency is only Phase 5, then immediately says capability detection needs Phase-1 interface definitions (\[implementation\_plan\_claude.md (line 413)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:413)). That is a dependency declaration error.

*   Phase 3 implements WithBeginNode/WithEndNode behavior in options.go, but Phase 4 later “finalize\[s\] WithBeginNode/WithEndNode as public options” (\[implementation\_plan\_claude.md (line 304)\](/Users/adc/dev/yama/implementation\_plan\_claude.md:304)). That ordering invites rework of Phase 3 boundary tests.

*   Building a generalized Phase 3 runtime runner before Phase 8 emission risks designing around handwritten “as-if-generated” fixtures instead of the actual generated code shape.


**4\. Definition Of Done Quality**

Issues by phase:

*   Phase 0: “empty-but-documented” is not a concrete acceptance condition.

*   Phase 1: “small identity value” is vague; it does not define the returned metadata type or fields.

*   Phase 2: “reported once” depends on unresolved A4; “panicking interceptor is contained per the chosen policy” has no chosen policy.

*   Phase 3: “caller context already canceled at Start” is listed with no expected observable result.

*   Phase 4: RunInBackground sink behavior and nil callback behavior assume an API not yet defined; EnsureExactlyOnce panic behavior is left as “document propagation vs containment.”

*   Phase 5/6/7/8/9 are mostly checkable. “clear/actionable error” is subjective but can be approximated with asserted message contents.

*   Phase 10: “proven by a deliberate tampering test in review, then reverted” is manual, not a durable test unless encoded in CI or a script.


**5\. Risk Miscalibration**

*   Phase 1 is marked MEDIUM, but it freezes public API, adds at least one API not clearly allowed (ComponentFromContext), and locks helper/constructor shapes. That is under-specified for an external compatibility surface.

*   Phase 2 is HIGH and correctly classified, but its DoD leaves overrun reporting and panic policy unresolved. That is not enough for a shared substrate used by every generated app.

*   Phase 3’s risk is framed as data integrity, but the bigger architectural risk is that it may violate the generated-code-first ADR by creating a handwritten orchestration engine.

*   Phase 8 is marked MEDIUM despite owning constructor semantics and cleanup discard behavior. A mistake there can double-teardown or strand cleanup. The DoD is better than the risk label.


**6\. Scope Creep Or Scope Gaps**

*   Scope creep: the handwritten runtime execution engine conflicts with the PRD/ADR direction that generated code contains lifecycle orchestration and runtime behavior is reduced to executing generated code.

*   Scope creep: ComponentFromContext is introduced as public API without appearing in ADR-007’s helper list.

*   Scope gap: operation identity metadata is absent from the plan.

*   No scope issues found around health checks, readiness frameworks, configuration, extra lifecycle phases, or alternate graph providers; the plan keeps those out.


**7\. Regression Blind Spots**

*   The regression map does not say to rerun the public API surface check after Phase 8/9 generated symbols are introduced.

*   Phase 5/6 parser and analysis changes are only broadly covered by Phase 10 drift. Drift does not prove behavioral correctness if wrong generated output is committed. It should explicitly rerun generated behavioral tests and goldens.

*   Phase 4 helper behavior changes only mention signature-driven golden regeneration. Behavioral changes in EnsureExactlyOnce, RunInBackground, or RunUntilSignal can break generated lifecycle behavior without changing signatures.

*   No regression chain covers lifecycle metadata/observability after Phase 8 emission, so the component/operation metadata gap could survive integration.