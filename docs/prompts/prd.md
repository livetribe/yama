# PRD Authoring Prompt: Compile-Time Go Lifecycle Framework

You are a senior Go architect and product manager.

Your task is to produce a comprehensive Product Requirements Document
(PRD) for a Go project whose purpose is compile-time lifecycle
orchestration for dependency-injected applications.

The PRD should be written as if it will be handed to an engineering team
for implementation.

## Project Summary

Design a Go framework that generates lifecycle orchestration code from a
compile-time dependency graph.

The project is inspired by Google Wire's philosophy:

-   Generate code.
-   Avoid reflection.
-   Avoid runtime registration.
-   Avoid service locators.
-   Avoid application frameworks.
-   Avoid owning the application runtime.
-   Keep the public API minimal.
-   Preserve strong typing.
-   Generate understandable Go code.

The framework is intentionally NOT a dependency injection container.

Dependency injection is handled by Google Wire.

This project focuses solely on lifecycle orchestration.

## Core Philosophy

The dependency graph already exists, created by Google Wire.

Applications currently maintain the same graph multiple times:

1.  Dependency construction.
2.  Startup ordering.
3.  Shutdown ordering.

This project should eliminate the duplication by generating lifecycle
orchestration from the dependency graph.

## Lifecycle Model

Supported lifecycle capabilities:

``` go
type Starter interface {
    Start(context.Context) error
}

type Drainer interface {
    Drain(context.Context) error
}

type Stopper interface {
    Stop(context.Context) error
}
```

Capabilities are optional.

Components implement whichever interfaces they need.

## Lifecycle Semantics

Startup:

-   Dependency-directed ordering.
-   Topological sort.
-   Independent branches start concurrently.
-   Fail fast on first startup failure.
-   No automatic rollback.
-   Application decides remediation.

Drain:

-   Concurrent.
-   No dependency ordering.
-   Purpose is to stop accepting new work.
-   Examples:
    -   Stop accepting HTTP requests.
    -   Stop consuming Kafka messages.
    -   Stop dequeuing jobs.

Shutdown:

-   Reverse dependency ordering.
-   Independent branches stop concurrently.
-   Components are responsible for flushing and cleanup during Stop().

## Explicit Non-Goals

Do NOT include:

-   Dependency injection.
-   Reflection.
-   Runtime registration.
-   Service locator APIs.
-   Readiness framework.
-   Health-check framework.
-   ConfigMap watching.
-   Runtime graph introspection.
-   Plugin system.
-   Component priorities.
-   Framework-owned retries.
-   Framework-owned backoff.
-   Framework-owned remediation.

## Configuration Philosophy

Configuration must be strongly typed.

Do not use string-keyed maps.

Generated code should create type-safe configuration structures.

Example:

``` go
type LifecycleConfig struct {
    KafkaConsumer KafkaConsumerConfig
    Router RouterConfig
}
```

Only generate configuration relevant to interfaces actually implemented
by a component.

For example:

-   Starter -\> Startup timeout.
-   Drainer -\> Drain timeout.
-   Stopper -\> Shutdown timeout.

Configuration precedence:

1.  Generated defaults.
2.  Code overrides.
3.  YAML / ConfigMap overrides.
4.  Validation.

Configuration is loaded once at startup.

No runtime reloading.

## Timeout Philosophy

Timeout policy is external.

Components do not define lifecycle policy.

Lifecycle system creates operation-specific contexts with deadlines.

Example:

``` go
ctx, cancel := context.WithTimeout(parent, timeout)
```

Component receives the context and may choose to inspect the deadline.

Timeouts include:

-   Startup timeout.
-   Drain timeout.
-   Shutdown timeout.

## Interceptors

Support lifecycle-specific interceptors.

Interceptors should resemble gRPC interceptor chains.

Interceptors are type-safe.

Avoid operation enums and string dispatch.

Possible interceptor methods:

-   BeforeStart
-   AfterStart
-   BeforeDrain
-   AfterDrain
-   BeforeStop
-   AfterStop

Interceptors are intended for:

-   Logging.
-   Telemetry.
-   Diagnostics.
-   Lifecycle policy.

Interceptors are NOT intended to become a generic middleware framework.

## Error Model

Lifecycle returns lifecycle-oriented errors.

It should not understand domain-specific errors.

Applications own remediation policy.

The framework reports:

-   Which component failed.
-   Which operation failed.
-   Original error.

## Generated Artifacts

Describe generated code in detail.

Potential outputs:

-   lifecycle_gen.go
-   lifecycle configuration structs
-   startup ordering
-   shutdown ordering
-   lifecycle plan generation

Generated code should be understandable Go code.

## Observability

Describe how lifecycle events should be observable through interceptors.

Examples:

-   Start duration.
-   Drain duration.
-   Stop duration.
-   Timeout events.
-   Failure events.

## DAG Analysis

The framework should analyze the dependency graph and compute:

-   Startup order.
-   Shutdown order.
-   Parallel execution groups.
-   Maximum theoretical startup time.
-   Maximum theoretical shutdown time.
-   Critical path analysis.

Explain how these values are calculated.

## Public API

Keep public APIs intentionally small.

Prefer a narrow, focused library rather than a framework.

## Deliverables

Produce:

1.  Vision.
2.  Problem statement.
3.  Goals.
4.  Non-goals.
5.  Architecture.
6.  Lifecycle model.
7.  Configuration model.
8.  Code generation model.
9.  Error model.
10. Interceptor model.
11. Timeout model.
12. DAG analysis model.
13. Observability model.
14. Example APIs.
15. Risks.
16. Open questions.
17. Future enhancements.
