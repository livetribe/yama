# Yama user guide

This guide is the behavior reference for applications that use Yama. It
covers the lifecycle model, what takes part in it, the order that it runs in,
its errors, its extension points, and the command that generates it. The
[README](../README.md) holds the quickstart and the rules for the files that
a run touches.
[`examples/hello`](../examples/hello) is a small, complete application.

## The lifecycle model

A component can implement any of three capabilities:

```go
type Starter interface {
	Start(context.Context) error
}

type Quiescer interface {
	Quiesce(context.Context)
}

type Stopper interface {
	Stop(context.Context)
}
```

`Start` brings a component into service. `Start` can fail. `Quiesce` stops
the intake of new work and waits for the work in flight. The component
defines what "new work" and "in flight" mean. `Stop` tears the component
down. `Quiesce` and `Stop` return nothing, because there is no recovery from
shutdown.

The generated constructor returns a `Lifecycle`:

```go
type Lifecycle interface {
	Start(context.Context) error
	Stop(context.Context)
}
```

There is no `Quiesce` method on `Lifecycle`. `Stop` runs the quiesce pass
first, unconditionally, and then the teardown pass. A caller who wants
shutdown calls `Stop`.

## Declaring lifecycles

A lifecycle stub declares each constructor, in a file behind
`//go:build yamainject`. The file follows Google Wire's `wireinject`
convention. A stub states the constructor's name, its signature, and the
graph's providers:

```go
//go:build yamainject

package hello

// NewLifecycle orchestrates the graph GraphSet builds, reporting to w.
func NewLifecycle(w io.Writer, opts ...yama.Option) (*Server, yama.Lifecycle, error) {
	panic(wire.Build(GraphSet))
}
```

The rules for a stub's signature:

- The results are exactly `(T, yama.Lifecycle, error)`. `T` is the value
  that the graph builds.
- The parameters are the graph's own arguments, exactly as a Wire injector
  takes them.
- A trailing option parameter is allowed and must be the variadic
  `opts ...yama.Option`. A constructor that accepts no options omits it.

A package can declare more than one stub. Each stub names one graph and
produces one constructor, so an application with two graphs declares two
stubs.

## What takes part in the lifecycle

Every component in the graph contributes to ordering. A component occupies a
level of its own in two cases. It is lifecycle-capable (its type implements
at least one of `Starter`, `Quiescer`, and `Stopper`), or its provider
returned a Google Wire cleanup function.

A component with neither trait receives no lifecycle calls. It still orders
the graph. When a component reaches a second component only through it, Yama
orders that component after the second component.

A cleanup runs at its component's position in the teardown pass, before that
component's own `Stop`. A cleanup takes no context and passes through no
interceptor chain. Yama supports it for compatibility with existing Wire
providers. Prefer the capability interfaces in new code.

## Ordering

Yama computes a dependency-ordered list of levels for each graph. Components
in the same level have no dependency relation, so they run concurrently
within a pass.

- **Start** walks the levels in dependency order. A component starts only
  after its dependencies started.
- **Stop** walks the levels twice in reverse: first the quiesce pass, then
  the teardown pass. A dependent quiesces and stops before its own
  dependencies.

Boundary components take the first and last positions in that order. Register them at the
call site with `WithBeginComponents` and `WithEndComponents`. A begin
component starts before every graph component, and it quiesces and stops
after every graph component. An end component starts after every graph
component, and it quiesces and stops before every graph component. The
[README](../README.md#boundary-components) shows the call.

## Startup failure

Startup is fail-fast. When a `Start` fails, Yama schedules no further level.
Components already in flight in the level where the `Start` failed run until
they finish. Yama does not cancel them.

Yama then runs the normal shutdown sequence (the quiesce pass, then the
teardown pass) over the components that started. The teardown pass also runs
the Google Wire cleanups of the levels that startup did not reach. The
components in those levels receive no lifecycle calls. `Start` then returns
the failure. The application does not call `Stop` after a failed `Start`. A call
that arrives anyway is a no-op.

## Errors

The public error surface is one value:

```go
var ErrStartFailed error
```

A failed `Start` returns an error that matches `ErrStartFailed` with
`errors.Is`. The error does not name the component that failed, and it does
not wrap the component's own error. `Stop` returns nothing. Component errors
are visible through interceptors. A start interceptor receives the error
that `next.Start` returns.

A component that panics in a lifecycle method counts as one that failed that
phase. Yama recovers the panic. After a `Start` panic, `Start` returns
`ErrStartFailed`. A `Quiesce` or `Stop` panic changes nothing for the
caller, and the pass continues to completion.

## Logging

Yama writes its own records through `log/slog`'s package-level default
logger:

- a recovered panic (Error, with the panic value and the stack)
- a quiesce or stop step skipped because the component's start failed (Warn)
- a component that returned after its context's deadline (Warn)

Yama has no logger configuration of its own. Set the `slog` default to
direct these records.

## Interceptors

Interceptors are the runtime extension point: logging, metrics, tracing,
policy. Each lifecycle operation has its own interceptor contract:

```go
type StartInterceptor interface {
	Start(ctx context.Context, next Starter) error
}

type QuiesceInterceptor interface {
	Quiesce(ctx context.Context, next Quiescer)
}

type StopInterceptor interface {
	Stop(ctx context.Context, next Stopper)
}
```

Register interceptors at the call site:

```go
app, lc, err := hello.NewLifecycle(os.Stdout, yama.WithInterceptors(&Logging{}))
```

The rules:

- A registered value joins the chain of each operation when the value
  implements that operation's interceptor interface. One value can implement
  all three.
- An interceptor runs once for each component call in its operation's pass,
  around that call. Interceptors run in registration order. The first
  registered interceptor is the outermost.
- An interceptor that calls `next` passes the operation on. An interceptor
  that does not call `next` suppresses the operation. The error that a start
  interceptor returns is that component's start failure.
- When a component's start failed, an interceptor does not see that
  component's quiesce or stop. Yama skips that component before the chain
  runs.

`FromContext` identifies the component under interception:

```go
func (l *Logging) Start(ctx context.Context, next yama.Starter) error {
	if component, ok := yama.FromContext[fmt.Stringer](ctx); ok {
		log.Printf("starting %s", component)
	}

	return next.Start(ctx)
}
```

Instantiate `FromContext` with a concrete type to scope an interceptor to
one component. Instantiate it with `any` to receive every component.

## The context

`Start` and `Stop` give the caller's context, unchanged, to every component
and every interceptor. Values on it reach all of them. A cancellation stops
the application in the same way that a signal does.

Yama observes a deadline on the context but does not enforce it. A
component's operation can run past the deadline. Yama does not abandon that
component. Yama waits, records the overrun once the operation returns, and
then proceeds. A component that wants a timeout of its own applies one
inside its own `Quiesce` or `Stop`.

## RunUntilSignal

A typical `main` calls `RunUntilSignal`:

```go
if err := yama.RunUntilSignal(context.Background(), lc); err != nil {
	log.Fatal(err)
}
```

It starts the lifecycle. It blocks until an OS signal arrives or the context
is done. It then stops the lifecycle and returns. When the caller names no
signals, `RunUntilSignal` waits on the interrupt and termination signals.
Pass specific signals to override that default. A signal that arrives during
startup takes effect once startup completes. `RunUntilSignal` ignores a
further signal during shutdown. A context that is done is not a failure. In
that case, `RunUntilSignal` returns nil.

## Running the generator

The command's observable behavior is `wire gen`'s, with one substitution.
Yama commits `lifecycle_gen.go` where Wire commits `wire_gen.go`. The flags
are Wire's own:

| Flag | Meaning |
| --- | --- |
| `-header_file` | file to insert as a header in the generated output |
| `-output_file_prefix` | string to prepend to output file names |
| `-tags` | build tags to append to the default build |

The package-pattern argument defaults to `.`. A named package that declares a
stub gets a `lifecycle_gen.go` and one progress line. The command passes over
a named package with no stub, writes no output for it, and exits 0, exactly
as `wire gen` passes over a package with no injector. A pattern that
resolves to no packages causes an error.

Generation is deterministic. A second run over an unchanged package produces
no diff. After a change to providers or stubs, regenerate and review the
diff of `lifecycle_gen.go` like any other code change. The
[README](../README.md#files-in-the-package-directory) states which files a
run writes, moves aside, and restores.
