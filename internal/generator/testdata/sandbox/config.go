// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sandbox

import (
	"fmt"
	"io"
)

// Environment names the deployment target the graph is built for.
type Environment string

const (
	Dev  Environment = "dev"
	Prod Environment = "prod"
)

// Config is application-wide configuration. It enters the graph as a Wire value
// (wire.Value); wire.FieldsOf then projects its Env field out as a standalone
// Environment dependency.
type Config struct {
	Env       Environment
	LogPrefix string
}

// Logger is the logging capability the components depend on. Wire binds it to
// *ConsoleLogger with wire.Bind, so components request the interface while Wire
// supplies the concrete type.
type Logger interface {
	Logf(format string, args ...any)
}

// ConsoleLogger writes prefixed lines to an io.Writer.
type ConsoleLogger struct {
	w      io.Writer
	prefix string
}

var _ Logger = (*ConsoleLogger)(nil)

// NewConsoleLogger builds a ConsoleLogger. The io.Writer arrives through
// wire.InterfaceValue (os.Stdout) or as an injector argument; the prefix comes
// from the injected Config.
func NewConsoleLogger(w io.Writer, cfg Config) *ConsoleLogger {
	return &ConsoleLogger{w: w, prefix: cfg.LogPrefix}
}

func (l *ConsoleLogger) Logf(format string, args ...any) {
	line := l.prefix + fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(l.w, line)
}
