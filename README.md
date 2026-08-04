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

## Usage

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

Then generate:

```bash
go generate ./...
```

Yama's flags and package-pattern argument are `wire gen`'s own, so a directive
that already names Wire's command carries over by changing the command it names.

The application calls the generated constructor and runs the lifecycle it
returns:

```go
app, lc, err := hello.NewLifecycle(os.Stdout)
if err != nil {
	log.Fatal(err)
}

if err := yama.RunUntilSignal(lc); err != nil {
	log.Fatal(err)
}
```

`lifecycle_gen.go` is the only file generation commits. Wire's `wire_gen.go`
and Yama's derived-injector file are transient intermediates, written into the
package directory and removed afterward. A file of either name that Yama did
not create is moved aside for the run and restored, so an application that
commits its own `wire_gen.go` keeps it.

[`examples/hello`](examples/hello) is a working application built this way. It
is its own Go module, so it reaches Yama the way an application does.

For the design, see:

* [`docs/PRD.md`](docs/PRD.md) — product requirements.
* [`docs/adr/`](docs/adr/) — the accepted architecture decision records.
* [`docs/Architecture.md`](docs/Architecture.md) — the resolved architecture,
  including the Public API Reference.
* [`implementation_plan_claude.md`](implementation_plan_claude.md) — the
  phased implementation plan.
