# ADR-005: Generated Lifecycle Configuration

## Status

Accepted

## Context

The lifecycle framework may require operation-specific configuration when invoking lifecycle methods.

Examples include:

* Startup timeouts.
* Drain timeouts.
* Shutdown timeouts.

Applications require a mechanism to customize lifecycle behavior without introducing:

* Configuration frameworks.
* Configuration loaders.
* Configuration parsers.
* Configuration validation frameworks.
* Runtime configuration management.
* Runtime configuration reloading.

The project intentionally focuses on lifecycle orchestration rather than configuration management.

A key architectural question is how lifecycle configuration should be represented and supplied to the generated lifecycle manager.

## Decision

The framework shall generate strongly-typed lifecycle configuration structures.

The generated configuration structures shall be consumed by the lifecycle manager.

The framework shall provide lifecycle configuration.

The framework shall not provide configuration management.

Specifically, the framework shall not provide:

* Configuration loading.
* Configuration parsing.
* Configuration discovery.
* Configuration validation frameworks.
* Configuration file formats.
* Environment variable binding.
* Flag binding.
* Runtime configuration reload.
* Configuration precedence systems.

The framework's responsibility begins after lifecycle configuration has been constructed by the application.

## Rationale

### Separation of Concerns

Lifecycle orchestration and configuration management are separate concerns.

The lifecycle framework should focus exclusively on:

* Lifecycle ordering.
* Lifecycle execution.
* Lifecycle configuration application.

Applications remain responsible for determining how lifecycle configuration is created.

### Avoiding Framework Expansion

Configuration management introduces numerous unrelated concerns:

* YAML support.
* JSON support.
* XML support.
* Environment variables.
* Flags.
* ConfigMaps.
* Validation.
* Reload semantics.
* Discovery mechanisms.

These concerns are orthogonal to lifecycle orchestration.

Supporting them would significantly expand the scope of the project.

### Strong Typing

Generated configuration structures preserve compile-time type safety.

Configuration is represented using ordinary Go types rather than:

* String keys.
* Maps.
* Reflection.
* Dynamic registries.

The compiler remains the primary validation mechanism.

### Integration Flexibility

Applications remain free to use any configuration system.

Examples include:

* Kubernetes ConfigMaps.
* Environment variables.
* YAML files.
* JSON files.
* Flags.
* Custom configuration systems.

The lifecycle framework remains agnostic to the source of configuration values.

## Generated Configuration Structures

The generator shall emit lifecycle configuration structures for lifecycle participants.

Configuration structures are generated per lifecycle participant rather than per Go type.

Example:

```go
type LCMConfig struct {
    Router        RouterConfig
    KafkaConsumer KafkaConsumerConfig
}
```

Configuration structures contain only lifecycle-related configuration.

The generator shall not generate application configuration.

## Interface-Driven Configuration Generation

Generated configuration fields depend on lifecycle capabilities implemented by a component.

Example:

```go
type RouterConfig struct {
    StartTimeout time.Duration
    StopTimeout  time.Duration
}
```

Because Router implements:

```go
Starter
Stopper
```

Example:

```go
type KafkaConsumerConfig struct {
    StartTimeout time.Duration
    DrainTimeout time.Duration
    StopTimeout  time.Duration
}
```

Because KafkaConsumer implements:

```go
Starter
Drainer
Stopper
```

Components that do not participate in lifecycle execution receive no generated lifecycle configuration.

Example:

```go
type Config struct{}
```

does not produce:

```go
type ConfigConfig struct{}
```

because Config does not implement any lifecycle capability interfaces.

## Configuration Ownership

Applications own configuration creation.

Example:

```go
cfg := LCMConfig{
    Router: RouterConfig{
        StartTimeout: 5 * time.Second,
    },
}

lcm := NewLifecycle(app, cfg)
```

The lifecycle framework consumes configuration but does not construct it.

The framework does not prescribe:

* Where configuration originates.
* How configuration is loaded.
* How configuration is persisted.

## Timeout Semantics

Lifecycle configuration may specify operation-specific timeouts.

Examples:

* Start timeout.
* Drain timeout.
* Stop timeout.

Timeouts are applied through context propagation.

The lifecycle framework shall never extend an existing context deadline.

The caller's context remains authoritative.

Conceptually:

```text
Caller Context Deadline
          ∩
Lifecycle Timeout
          ↓
Effective Deadline
```

Lifecycle configuration may shorten a deadline.

Lifecycle configuration may not lengthen a deadline.

## Error Semantics

Timeout expiration is treated as a normal lifecycle failure.

The framework does not introduce timeout-specific remediation behavior.

The framework does not introduce timeout-specific recovery behavior.

Timeout failures participate in normal lifecycle error handling.

## Third-Party Configuration Integration

The generated configuration structures are intentionally designed to be consumable by external configuration systems.

Examples include:

* YAML loaders.
* JSON loaders.
* Environment variable binders.
* Flag parsers.

However, these systems exist outside the lifecycle framework.

The lifecycle framework consumes configuration structures but does not provide mechanisms for constructing them.

## Generated Configuration Is Not Public API

Generated lifecycle configuration structures are application-specific generated artifacts.

Examples:

```go
type LCMConfig struct {
    ...
}
```

These structures are generated from a specific dependency graph.

They are not part of the lifecycle library's stable public API.

The lifecycle library consumes them but does not define them.

## Consequences

### Positive

* Strongly typed lifecycle configuration.
* No configuration framework.
* No configuration format support.
* No runtime configuration management.
* Clear separation of concerns.
* Easy integration with existing configuration systems.

### Negative

* Applications must provide configuration values themselves.
* Applications are responsible for any desired validation.
* Applications are responsible for configuration loading.

### Accepted Trade-Off

The project prioritizes lifecycle orchestration over configuration management.

The framework intentionally delegates configuration concerns to applications and external libraries.

## Rejected Alternatives

### Functional Options

Example:

```go
WithRouterStartTimeout(...)
WithRouterStopTimeout(...)
WithKafkaStartTimeout(...)
WithKafkaDrainTimeout(...)
```

Rejected because large applications may generate excessive numbers of lifecycle options.

Generated configuration structures provide a more scalable representation.

### Configuration Framework

Rejected because configuration acquisition, parsing, validation, and reload semantics are outside the scope of lifecycle orchestration.

### String-Keyed Configuration

Example:

```go
map[string]time.Duration
```

Rejected because it sacrifices type safety and discoverability.

### Runtime Configuration Reload

Rejected because lifecycle configuration is evaluated during lifecycle execution and does not require runtime management.

## Non-Goals

This decision does not provide:

* YAML support.
* JSON support.
* XML support.
* Environment variable support.
* Flag support.
* ConfigMap support.
* Validation frameworks.
* Runtime configuration reload.
* Configuration precedence systems.

The lifecycle framework consumes lifecycle configuration but does not manage configuration.
