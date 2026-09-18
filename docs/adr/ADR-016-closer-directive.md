# ADR-016: The Closer Directive Binds a Provider's `Close` to the Teardown Pass

## Status

Proposed

## Context

A component receives lifecycle calls when its type implements `Starter`,
`Quiescer`, or `Stopper` (ADR-003). Yama detects each capability from the
type's method set. That rule works for a type that the application owns.
The application adds the method that it needs.

Many components come from a package that the application does not own.
`*sql.DB`, `*grpc.ClientConn`, and `*redis.Client` are examples. Each
implements `io.Closer`, and none implements `Stopper`. The application
cannot add a method to a type that it does not own. Yama calls only the
capability methods, so nothing calls the component's `Close`.

The application has two ways to close such a component today. Both cost
boilerplate code that the application must write for each closer.

The first way is a provider that returns a cleanup:

```go
func NewDB(cfg Config) (*sql.DB, func (), error) {
db, err := sql.Open(cfg.Driver, cfg.DSN)
if err != nil {
return nil, nil, err
}
return db, func () { _ = db.Close() }, nil
}
```

A cleanup is not a lifecycle capability. It takes no context and passes
through no interceptor chain, and Yama supports it for compatibility with
existing Google Wire code (ADR-008).

The second way is an adapter type that the application writes. The adapter
embeds the third-party value and declares `Stop`, and a provider returns the
adapter. The adapter is a `Stopper`, so its teardown runs under the pass
context and through the interceptors. This costs a type, a method, and a
provider per closer. It also changes the graph. The adapter replaces the
third-party type as the component, and every consumer must depend on the
adapter.

## Decision

A directive goes on what builds the component. A provider function builds
most components, and its declaration carries the directive. A later section
covers a component that no provider function builds.

An application marks a provider as a closer with the **closer directive**.
The directive is the comment `//yama:closer` in the doc comment of the
provider's declaration:

```go
// NewDB provides the database handle.
//
//yama:closer
func NewDB(cfg Config) (*sql.DB, error) {
return sql.Open(cfg.Driver, cfg.DSN)
}
```

A component that a marked provider builds is a **closer component**. When
Yama adds the closer component to its level, it wraps the component in a
`Stopper`. The wrapper's `Stop` calls the component's `Close`. The wrapper
exists only in the level. The provider still returns the component itself,
and every consumer receives it. A closer component can already be
lifecycle-capable, because its type can declare `Start` or `Quiesce`. It
cannot declare `Stop`. The wrapped component is a lifecycle-capable
component like any other. It occupies a level, and it takes part in the
teardown pass. Its `Stop` runs under the pass context and passes through the
`Stop` interceptor chain (ADR-005). The context carries the component, not
the wrapper. When `Close` returns an error, the wrapper logs the error, and
the teardown pass continues.

### The warning for an unmarked closer

Yama prints a warning for a component when all of these conditions are
true:

* a provider function or a `wire.Struct` entry builds the component in a
  graph;
* the component's type implements `io.Closer`;
* what builds the component carries no directive;
* the component has no cleanup;
* the component's type does not implement `Stopper`.

The second condition reads the type of the component that the graph built.
A graph can build a value when only the pointer type declares `Close`. That
value does not implement `io.Closer`, so Yama prints no warning for it.

The warning names what builds the component and the position of that
declaration or entry. Yama prints one warning for each such provider or
entry. A warning does not stop generation, and it does not change the
command's exit status.

ADR-012 makes the command's output match the output of `wire gen`, unless
another ADR states a difference. `wire gen` prints no warning. This ADR
states that difference. The warning is one line on standard error. The line
starts with `yama:`, then the package path, then `warning:`, in the form of
Yama's progress line.

A `Quiescer` or a `Starter` does not stop the warning. Neither method closes
the component.

### The noclose directive

An application stops the warning with the **noclose directive**. The
directive is the comment `//yama:noclose` in the doc comment of the
provider's declaration:

```go
// NewOutput provides the stream that the application writes to.
//
//yama:noclose
func NewOutput() *os.File {
return os.Stdout
}
```

The directive states that the lifecycle does not close the component. It
stops the warning, and it has no other effect. Yama does not wrap the
component, and nothing calls its `Close`.

### Placement

The two directives share one placement. A directive is a comment of the
exact form `//yama:closer` or `//yama:noclose`, with no space after the
slashes. It is one line of the doc comment of a function declaration. The
function has no receiver, because a provider is never a method. `gofmt` puts
a directive line at the end of the doc comment. When the doc comment has
other text, `gofmt` puts a blank comment line before the directive.

### A component that no provider function builds

A `wire.Struct` entry builds a component with no provider function. Such a
component has no declaration to carry a directive. Its directive goes on
the entry, in the argument list of a `wire.NewSet` call. The directive
occupies its own line, immediately before the entry:

```go
var GraphSet = wire.NewSet(NewConfig, NewServer,
//yama:closer
wire.Struct(new(Conn), "*"))
```

One `wire.Struct` entry lets a graph build the struct as a value, as a
pointer, or as both. In Google Wire's injector, each such component is a
struct literal of the type that the entry names, or a pointer to such a
literal. A graph that builds both forms holds two components. Yama matches
a marked entry to each component of either form. It wraps each one with a
type that implements `io.Closer`, and it does not wrap the others.

A marked entry marks its struct type for the package. It applies to every
graph of the package that builds that struct type from a `wire.Struct`
entry.

A directive on an entry that names a provider function is an error. That
directive goes on the provider's declaration.

### What Yama does for each form of a component

The table states what Yama does for each form that a graph can build. `T` is
a type, and `*T` is a pointer to it. A provider function builds one
form. A `wire.Struct` entry can build one form or both.

| App built    | `Close` is on      | With `//yama:closer`                 | With `//yama:noclose` | With no directive |
|:------------:|:------------------:|--------------------------------------|-----------------------|-------------------|
| `T`          | `T`                | wrap it                              | silent                | warn              |
| `T`          | `*T`               | error                                | error                 | silent            |
| `*T`         | either `T` or `*T` | wrap it                              | silent                | warn              |
| `T` and `*T` | either or both     | wrap ones that implement `io.Closer` | silent                | warn              |
| `T` and `*T` | neither            | error                                | error                 | silent            |

### How Yama reads a directive

Yama learns a graph from the injector that Google Wire writes (ADR-008). A
statement that builds a component from a provider calls that provider
function. Yama collects the providers that the graphs call. It loads the
package of each provider by import path, with its source, and it reads the
doc comment of the provider's declaration. The provider can be in the
package that Yama generates for, in another package of the application, or
in a package of another module.

A marked provider is a closer in every graph that calls it. The provider
set that lists the provider has no effect.

Yama reads a directive on a set entry from the `wire.NewSet` calls in the
package that it generates for (ADR-014). It does not read the provider sets
of any other package.

### Checks

Yama reports an error, and generates nothing for the package, in three
groups of cases.

The first group covers the package that Yama generates for. Yama reads every
file of that package that the run's build tags select. It reports:

* a `//yama:` comment that is not in the doc comment of a function
  declaration, and not on its own line immediately before an entry of a
  `wire.NewSet` call;
* a `//yama:` comment in the doc comment of a method;
* a directive on an entry that is not a `wire.Struct` entry;
* two `wire.Struct` entries that name one struct type and that do not carry
  the same directive.

The second group covers the doc comment of each provider that a graph
calls, in any package. Yama reports:

* a `//yama:` comment with a name other than `closer` or `noclose`;
* a provider that carries both directives.

The third group covers each provider and each entry that carries a
directive and that builds a component in a graph.

For `//yama:closer`, Yama reports:

* a provider or an entry that builds components in a graph, when no such
  component has a type that implements `io.Closer`. This covers a graph that
  builds only a value when only the pointer type declares `Close`;
* a provider that also returns a cleanup;
* a component that Yama wraps, with a type that already implements
  `Stopper`.

For `//yama:noclose`, Yama reports a provider or an entry that builds
components in a graph, when no such component has a type that implements
`io.Closer`.

Yama also reports an error when it cannot load the package of a provider.

### The runtime-support package supplies the wrapper

The runtime-support package (ADR-010) exports one function that wraps a
closer in a `Stopper`. The generated constructor calls that function where
it adds the component to its level:

```go
WithComponents(rt.AsStopper(db))
```

The wrapper passes `Start` and `Quiesce` to a component that declares them.

## Rationale

### Yama supplies the capability that the type lacks

Yama detects a capability from a type's method set. A third-party type has a
method set that the application cannot change. The application usually owns
the provider that builds the component, and a later section states why. A
fact that the application states on that provider is under its control. On
that fact, Yama supplies the `Stop` that the type lacks.

The lifecycle model does not change. A wrapped closer is a `Stopper`. Each
rule that an ADR states for a lifecycle-capable component applies to it
with no exception: level occupancy, ordering, the gates, and interception.

### The provider function is the link that Yama has

Google Wire's injector names the provider function that builds each
component. It does not name the provider set that listed the function. The
provider function is therefore the only link from a component in the graph
to a declaration that the application owns. A directive on the function's
declaration is a fact about that link. Yama needs no further rule to decide
which graphs the directive applies to.

### A directive goes on what builds the component

Each component has one source in a graph: a provider function or a set
entry. The directive is a fact about how the lifecycle treats what that
source builds, so it goes on that source. A provider function has a
declaration, and the declaration carries the directive. A `wire.Struct`
entry has no declaration, so the entry carries it. One rule gives each of
these components one place for its directive.

A `wire.Struct` entry gives Yama no function to match. The struct type that
the entry names is the link. Google Wire writes that type into the struct
literal that builds the component, so Yama reads the same type on both
sides. The check for two entries of one struct type keeps that link exact
in a package. With no such check, a directive in one set marks a type that
a second set builds with no directive, and a reader of the second set
cannot see the mark.

A `wire.Value` entry and a `wire.InterfaceValue` entry carry no directive.
The application makes such a value outside the graph, so the graph does not
own it. The type is also not a usable link for these entries. A
`wire.InterfaceValue` entry names an interface, and the component in the
injector has the concrete type of the value.

### The application already owns the provider

Google Wire injects by type. A third-party constructor usually takes values
that have no distinct type in the graph. For example, `sql.Open` takes two
strings, and `grpc.NewClient` takes a target string and options. The
application therefore writes a provider of its own that takes its
configuration and calls the constructor. That provider is where the
directive goes. The directive adds one line to a function that the
application writes for another reason.

### Google Wire does not change

Google Wire reads a provider's signature. It does not read the provider's
doc comment. A directive there changes nothing that Google Wire reads. As a
result, the derived injector (ADR-011) and Google Wire's output are the
same with and without it.

### `Close` runs as a `Stop`, not as a cleanup

A cleanup runs with no context, outside the interceptor chains, and with no
component identity (ADR-008). A closer's teardown has a component, a
position in the levels, and a position in the teardown pass. A `Close`
bound to the teardown pass has the context, the interceptors, and the
identity that every other component teardown has. This binding also makes
the directive the primary path for a third-party closer, and leaves the
cleanup as the compatibility path that ADR-008 describes.

`Stop` returns no error, so a `Close` error has no return path. ADR-006
treats a panic during `Stop` as a diagnostic. Yama logs it, the pass
continues, and no caller receives it. A `Close` error follows the same
rule. It is a diagnostic about one component, not a result of the pass.

### A warning finds the closer that the application forgot

The closer directive is opt-in. An application that forgets it gets no
`Close` and no signal. Yama already holds each fact that the warning needs:
the provider's result type, its results, and its doc comment. The warning
therefore costs one more read of facts that Yama loaded for another reason.

### The warning is not an error

An error makes the directive mandatory for every closer. An application
that generated with no failure before this decision would fail after it,
with no change on its side. A warning leaves each application as it was and
adds a line to the output.

### The warning needs an opt-out

Some closers must not be closed by the lifecycle. A provider can return
`os.Stdout`. A provider can return a field of a dependency, and the owner
of that dependency closes it. The application can close a shared handle in
another place. A warning with no opt-out repeats on every run for each of
these. A reader then learns to ignore Yama's output, and a later warning
about a closer that the application did forget goes unread. The noclose
directive records the decision at the provider, where the next reader
finds it.

### A cleanup or a `Stop` shows that the application wrote a teardown

A cleanup or a `Stop` is teardown code that the application wrote for the
component. Yama does not inspect that code for a call to `Close`. `Quiesce`
and `Start` are different. `Quiesce` stops new work and `Start` begins
work (ADR-003). Neither one is a teardown, so neither one stops the
warning.

### The placement follows the Go directive convention

A Go directive that describes a function sits in the function's doc
comment. `//go:noinline` and `//go:linkname` take that position. A comment
of the form `//yama:closer` has the syntax of a directive, so the Go tools
treat it as one. `gofmt` keeps it as the last line of the doc comment, and
`go doc` does not print it.

## Consequences

### Positive

One line marks a closer, on a provider that the application already owns.

The directive has one meaning in every graph and in every package. A
provider in a shared package carries its directive to each application
package that calls it.

A third-party closer's teardown runs under the pass context and through the
`Stop` interceptors. An interceptor observes it like any other component
teardown.

Google Wire, the derived injector, and Google Wire's output do not change.

A closer that the application forgot becomes visible on the next run. A
closer that the application decided not to close carries that decision in
its provider's doc comment.

### Negative

Yama gains a second way to make a component lifecycle-capable. The type's
method set is the first. A directive on the provider is the second.

The generator must read provider declarations. It loads the source of each
package that declares a provider, in addition to the package that it
generates for.

The runtime-support package's public surface grows by the one function
that wraps a closer.

An application has three ways to tear down a third-party value: the
directive, a cleanup, and an adapter type. The guide must state which one
to use and when.

The command's output differs from `wire gen`'s output for the first time
in a way that is not an error. A script that treats any line on standard
error as a failure now fails on a warning.

An application with unmarked closers sees new warnings on its first run
after this decision. Each one needs a directive or a decision to ignore it.

The compiler does not check a directive. A misspelled directive name is an
error at generation time, not at compile time.

### Accepted Trade-Off

When a closer carries no directive, and the application writes no teardown
of its own for it, nothing calls its `Close`. The warning reports that
closer, and the application can ignore the warning. Yama does not make the
decision for the application.

A provider that the application does not own has no doc comment that the
application can edit. When a provider set lists such a provider directly,
the application writes a provider of its own to carry either directive.
Until it does, the warning for that provider repeats on each run. The
Rationale states why that case is not the usual one.

Yama reads set entries only in the package that it generates for. A set in
another package can hold a `wire.Struct` entry that builds a closer. The
application cannot put a directive on that entry. It can mark the struct
type with a `wire.Struct` entry in a set of its own package, or it can
replace the entry with a provider function. A mark on a struct type also
reaches a graph that builds the type from the other package's entry. Yama
cannot tell the two entries apart, because the injector names the type and
not the entry.

Three kinds of component get no warning and carry no directive. A
`wire.Value` entry and a `wire.InterfaceValue` entry supply a value that the
application made outside the graph. A `wire.FieldsOf` entry takes
components from the fields of a struct, and the struct that holds a field
owns it.

Yama reports a misplaced directive only in the package that it generates
for. A misplaced directive in any other package has no effect, and Yama
does not report it.

An unknown directive name on a provider in a package that the application
does not own stops generation. The application cannot edit that file. It
can write a provider of its own that calls the third-party provider.

## Rejected Alternatives

### A directive on a provider function's entry in the set

The directive goes on the provider's entry in a `wire.NewSet` call, in
place of the provider's declaration:

```go
var GraphSet = wire.NewSet(NewConfig, NewStore, NewServer,
//yama:closer
NewDB)
```

A third-party provider goes into the set as it is, and the application
writes no provider of its own. Rejected, for three reasons.

The placement states a scope that Yama cannot apply. A directive on one
entry of one set reads as a fact about that set. The Rationale states why
Yama cannot tell which set a graph reached a provider through. A directive
in one set therefore marks the provider in every graph of the package. One
such graph can come from a second set that lists the provider with no
directive.

Yama reads the source of the package that it generates for, and of each
package that declares a provider. A provider set can be in a third package.
A directive in that set has no effect.

The case that this form served is not the usual one. The Rationale states
why an application usually owns the provider of a third-party closer.

The decision does put a directive on a set entry, but only on a
`wire.Struct` entry. No declaration exists for that component, and a check
keeps the link to the struct type exact in the package. A provider function
has a better place.

The authors also considered two variants of this form. One named the
provider in a directive on the set variable, and one put the directive at
the end of the entry's line. Both share the three defects.

### Treat every `io.Closer` as a `Stopper`

Yama binds `Close` to the teardown pass for every component that implements
`io.Closer` and not `Stopper`. The application writes no directive. Rejected,
because the tool then decides which values to close, and it decides wrongly
in common cases.

`*http.Server` implements `io.Closer`. Its `Close` stops requests that are
in progress, and its graceful method is `Shutdown`. Some providers return a
value that they did not build, for example a field of a dependency. The
provider does not own that closer. The owner closes it, and Yama closes it
again. A provider that returns `os.Stdout` as an `*os.File` returns a closer
that no component must close. A dependency that adds `Close` in a minor
release changes the application's shutdown with no change on the
application's side.

Each case needs an opt-out, so this alternative is an opt-out directive
compared with an opt-in one. The line count is about the same. The
difference is how the silent default fails. Opt-in fails as a leak, which
is the state today. Opt-out fails as a wrong close, which the framework
would introduce.

### Report an unmarked closer as an error

Yama stops generation for a closer that carries no directive, no cleanup,
and no `Stop`. The noclose directive is the only way past the error.
Rejected. The Rationale states why the report is a warning.

### Keep the cleanup provider as the pattern

The application returns a cleanup from the provider of each third-party
closer, and the guide documents it as the pattern. Rejected, because it
makes the compatibility path the pattern for a common case. The Rationale
states what a cleanup lacks.

### Keep the adapter type as the pattern

The application writes an adapter type for each third-party closer, and
the guide documents it as the pattern. The adapter gives the teardown the
context and the interceptors that a cleanup lacks. Rejected, because it
costs the most code of any option, and because it changes the graph. The
third-party type is no longer a component, and every consumer of it must
change its dependency to the adapter type.

The decision also wraps the component, with one difference. Yama's wrapper
exists only in the level, after construction. The graph keeps the
third-party type, and no consumer changes.

### A marker function inside the set

The application wraps the provider in a marker call inside the set, in the
style of `wire.Bind`:

```go
var GraphSet = wire.NewSet(NewConfig, NewStore, NewServer,
yama.WrapCloser(NewDB))
```

Rejected, because Google Wire rejects a call inside `wire.NewSet` that is not
one of its own directives. This form needs a change to Google Wire.

### A marker function in the provider body

The provider returns the value through an identity function that Yama
looks for in the body:

```go
func NewDB(cfg Config) (*sql.DB, error) {
db, err := sql.Open(cfg.Driver, cfg.DSN)
return yama.WrapCloser(db), err
}
```

Google Wire sees an ordinary provider. Rejected, because the generator must
inspect function bodies. A marker in a return statement is easy to find. A
marker in an assignment, a helper, or one of two return branches needs
data-flow analysis. The generator finds no marker in any other position and
reports nothing. A directive on the declaration states the same fact and
needs no analysis of the body.

## Non-Goals

This ADR does not bind any method other than `Close`. It does not add a
directive for `Start` or `Quiesce`, and it does not add a general form that
names a method to bind.

This ADR does not make an unmarked closer an error. Yama calls `Close` only
on a closer component.

This ADR does not add a command flag that hides the warning or that makes
it an error.

This ADR does not change Google Wire.

This ADR does not read a directive in a `wire.Build` call. An entry that
carries a directive goes in a `wire.NewSet` call.

This ADR does not cover a component from a `wire.Value`, a
`wire.InterfaceValue`, or a `wire.FieldsOf` entry. Such a component gets no
warning and no directive.

This ADR does not define how Yama treats `Close` on a type with a separate
graceful method, such as `*http.Server`. The application decides whether to
mark such a provider.
