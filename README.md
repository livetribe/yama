# yama

A compile-time lifecycle orchestration framework: it derives application
startup/quiesce/shutdown ordering from a Google Wire dependency graph and
generates the orchestration code, rather than building a runtime engine that
interprets one.

[![Build Status](https://github.com/livetribe/yama/actions/workflows/ci.yml/badge.svg)](https://github.com/livetribe/yama/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/livetribe/yama)](https://goreportcard.com/report/github.com/livetribe/yama)
[![Documentation](https://godoc.org/l7e.io/yama/v2?status.svg)](http://godoc.org/l7e.io/yama/v2)
[![Coverage Status](https://coveralls.io/repos/github/livetribe/yama/badge.svg?branch=v2)](https://coveralls.io/github/livetribe/yama?branch=v2)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

![Image of Yama](https://github.com/livetribe/yama/raw/master/img/yama.jpg)

## Status

This is v2, a green-field rewrite; it shares only a name and a repository
with the earlier `v0.x` signal-watcher. v2 is under active construction: the
public API surface is defined and frozen, but the code generator and runtime
are still being built.

## Setup

Generation is one command. Yama runs Google Wire for you, reads the injector
Wire produces, and writes `lifecycle_gen.go`.

Pin both tools in the application's `go.mod`. Yama invokes Wire as
`go tool wire` from the target package's own module, so that module supplies
it:

```
tool (
	github.com/google/wire/cmd/wire
	l7e.io/yama/v2/cmd/yama
)
```

## Declaring a lifecycle

Declare the graph with ordinary Wire providers. Then add a lifecycle stub file,
behind the `yamainject` build tag, naming each constructor the application
calls and the providers its graph is built from:

```go
//go:build yamainject

package hello

// NewLifecycle orchestrates the graph GraphSet builds, reporting to w.
func NewLifecycle(w io.Writer, opts ...yama.Option) (*Server, yama.Lifecycle, error) {
	panic(wire.Build(GraphSet))
}
```

Add the generate directive to a committed file that no build tag excludes;
`go generate` does not read a file a build tag hides, so it cannot live in the
stub file:

```go
//go:generate go tool yama
```

## Generating

Then generate:

```bash
go generate ./...
```

Yama's flags and package-pattern argument are `wire gen`'s own, so a directive
that already names Wire's command carries over by changing the command it names.

## Running

The application calls the generated constructor and runs the lifecycle it
returns:

```go
app, lc, err := hello.NewLifecycle(os.Stdout)
if err != nil {
	log.Fatal(err)
}

if err := yama.RunUntilSignal(context.Background(), lc); err != nil {
	log.Fatal(err)
}
```

`RunUntilSignal` gives its context to `Start` and to `Stop` without a change.
Every component and every interceptor therefore receives it. Deployment facts on
that context, such as a node identifier, reach all of them. A cancellation and a
deadline reach them too. A cancellation also stops the application in the same way
that a signal does.

## Boundary components

Some components belong at the edges of the graph's order rather than inside it.
Two options on the generated constructor register them:

```go
app, lc, err := hello.NewLifecycle(os.Stdout,
	yama.WithBeginComponents(telemetry),
	yama.WithEndComponents(readiness),
)
```

A begin component starts before every graph component, and it quiesces and
stops after every graph component. Base services such as telemetry belong
here. They outlive everything that uses them.

An end component starts after every graph component, and it quiesces and
stops before every graph component. A readiness flip belongs here. It turns
on only when the whole application is up, and it turns off first.

A boundary component participates through the capabilities it implements,
exactly as a graph component does. Both options are variadic and accumulate
across calls.

## Files in the package directory

`lifecycle_gen.go` is the only file generation commits. A run also writes two
transient files into the package directory and removes both before it returns:

| File | Owner | Committed |
|---|---|---|
| `lifecycle_gen.go` | Yama | yes |
| `wire_gen.go` | Wire | no, transient |
| `yama_wireinject.go` | Yama | no, transient |

A run does not overwrite a `wire_gen.go` that it did not create. It moves that
file to `.yama.wire_gen.go` for the run, and puts it back at the end. An
application that commits its own Wire output therefore keeps it. A run moves a
committed `lifecycle_gen.go` to `.yama.lifecycle_gen.go` the same way.

Yama owns the name `yama_wireinject.go`. A run writes over a file already at
that name. Do not keep a file of your own there.

Do not start two runs over one package directory at the same time. Yama does
not lock the directory. The second run's cleanup can delete a `wire_gen.go`
that the first run put back.

## Recovering from an interrupted run

A run that stops before it completes does not reach its cleanup. It can leave
these files behind:

* `.yama.wire_gen.go` — your `wire_gen.go`, if you committed one.
* `.yama.lifecycle_gen.go` — your committed `lifecycle_gen.go`.
* `wire_gen.go` — Wire's output from that run, not yours.
* `yama_wireinject.go` — Yama's derived injectors.

Generate again. The next run repairs the directory before it reads anything:

```bash
go generate ./...
```

It puts both `.yama.` files back under their original names first, and discards
the output the interrupted run left. You lose nothing. Yama creates a `.yama.`
file only to hold a file of yours.

## Example

[`examples/hello`](examples/hello) is a working application built this way. It
is its own Go module, so it reaches Yama the way an application does.

## Guide

[`docs/guide.md`](docs/guide.md) is the behavior reference: the lifecycle
model, ordering, errors, interceptors, the context, and the command.

## Design

For the design, see:

* [`docs/PRD.md`](docs/PRD.md) — product requirements.
* [`docs/adr/`](docs/adr/) — the accepted architecture decision records.
* [`docs/Architecture.md`](docs/Architecture.md) — the resolved architecture,
  including the Public API Reference.
* [`implementation_plan_claude.md`](implementation_plan_claude.md) — the
  phased implementation plan.
