# hello

`hello` is a small Yama application. It is a separate Go module, so it uses
Yama in the same way that an application does.

The graph has four components:

```text
server → client → store → config
```

* `Config` has no lifecycle capability. It is only a dependency.
* `Store` starts and stops. Its provider also returns a cleanup function.
* `Client` declares only `Close`. The closer directive on its provider binds
  `Close` to the teardown pass.
* `Server` starts, quiesces, and stops.

## Running

```bash
go run ./cmd/hello
```

The command runs until it receives an interrupt signal or a termination
signal.

`lifecycle_gen.go` is the only generated file that this package commits. It
has its own `go:generate` directive. To generate `lifecycle_gen.go` again, run:

```bash
go generate ./...
```

## `components.go`

`components.go` declares the graph with Google Wire providers. Each `var _`
line makes the compiler check that a component has a lifecycle capability.

```go
package hello

import (
	"context"
	"fmt"
	"io"

	"github.com/google/wire"

	"l7e.io/yama"
)

// GraphSet builds the whole graph from the io.Writer the lifecycle constructor
// takes as an argument.
var GraphSet = wire.NewSet(NewConfig, NewStore, NewServer, NewClient)

// The capabilities each component carries. Config declares none. Client
// declares Close, and the closer directive on NewClient binds it.
var (
	_ yama.Starter = (*Store)(nil)
	_ yama.Stopper = (*Store)(nil)

	_ yama.Starter  = (*Server)(nil)
	_ yama.Quiescer = (*Server)(nil)
	_ yama.Stopper  = (*Server)(nil)

	_ io.Closer = (*Client)(nil)
)

// Config is the application's configuration. It implements no lifecycle
// capability, so it is a dependency of the graph and not a member of any level.
type Config struct {
	Greeting string
}

// NewConfig provides the Config.
func NewConfig() Config {
	return Config{Greeting: "hello"}
}

// Store holds the application's data. Its provider returns a cleanup function,
// which runs at the store's own position in the teardown, before Stop.
type Store struct {
	w   io.Writer
	cfg Config
}

// NewStore provides the Store and the cleanup that releases what it holds.
func NewStore(w io.Writer, cfg Config) (store *Store, cleanup func()) {
	store = &Store{w: w, cfg: cfg}
	cleanup = func() {
		fmt.Fprintln(store.w, "store: released")
	}

	return store, cleanup
}

func (s *Store) Start(context.Context) error {
	fmt.Fprintln(s.w, "store: open")

	return nil
}

func (s *Store) Stop(context.Context) {
	fmt.Fprintln(s.w, "store: closed")
}

func (s *Store) String() string {
	return "store"
}

// Server serves requests from the Store through the Client. It depends on
// both, so it starts after them and stops before them.
type Server struct {
	w      io.Writer
	store  *Store
	client *Client
}

// NewServer provides the Server.
func NewServer(w io.Writer, store *Store, client *Client) *Server {
	return &Server{w: w, store: store, client: client}
}

func (s *Server) Start(context.Context) error {
	fmt.Fprintf(s.w, "server: serving %q\n", s.store.cfg.Greeting)

	return nil
}

// Quiesce stops accepting new work. It runs before any component's Stop, so the
// store is still open while the server drains.
func (s *Server) Quiesce(context.Context) {
	fmt.Fprintln(s.w, "server: draining")
}

func (s *Server) Stop(context.Context) {
	fmt.Fprintln(s.w, "server: stopped")
}

func (s *Server) String() string {
	return "server"
}
```

## `client.go`

`Client` represents a type from a package that the application does not own.
The `//yama:closer` directive on `NewClient` binds `Client.Close` to the
teardown pass.

```go
package hello

import (
	"fmt"
	"io"
)

// Client stands in for a type from a package that the application does not
// own. It declares Close and no lifecycle capability. The closer directive on
// its provider binds Close to the teardown pass.
type Client struct {
	w     io.Writer
	store *Store
}

// NewClient provides the Client. It depends on the Store, so it closes before
// the store stops.
//
//yama:closer
func NewClient(w io.Writer, store *Store) *Client {
	return &Client{w: w, store: store}
}

// Close releases what the client holds.
func (c *Client) Close() error {
	fmt.Fprintln(c.w, "client: closed")

	return nil
}
```

## `lifecycle.go`

`lifecycle.go` is the lifecycle stub. The `yamainject` build tag excludes it
from a normal build. Yama reads the stub and writes `lifecycle_gen.go`. The
generated file declares a constructor with the same signature.

```go
//go:build yamainject

package hello

import (
	"io"

	"github.com/google/wire"

	"l7e.io/yama"
)

// NewLifecycle orchestrates the graph GraphSet builds, reporting to w.
func NewLifecycle(w io.Writer, opts ...yama.Option) (*Server, yama.Lifecycle, error) {
	panic(wire.Build(GraphSet))
}
```

## `cmd/hello/main.go`

`main` calls the generated constructor. Then it gives the lifecycle to
`yama.RunUntilSignal`.

```go
// Command hello runs the example application until it is interrupted.
package main

import (
	"context"
	"log"
	"os"

	"l7e.io/yama"

	"example.com/hello"
)

func main() {
	app, lc, err := hello.NewLifecycle(os.Stdout)
	if err != nil {
		log.Fatalf("hello: construction failed: %v", err)
	}

	log.Printf("hello: built %s", app)

	if err := yama.RunUntilSignal(context.Background(), lc); err != nil {
		log.Fatalf("hello: %v", err)
	}
}
```

## Output

`Start` starts the store. Then it starts the server. When a signal arrives,
`Stop` quiesces the server first. Then the components stop in the reverse of
the dependency order. The store's cleanup runs before the store's `Stop`. The
components write these lines to standard output:

```text
store: open
server: serving "hello"
server: draining
server: stopped
client: closed
store: released
store: closed
```
