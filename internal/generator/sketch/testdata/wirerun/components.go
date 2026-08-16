package wirerun

import (
	"context"

	"gopkg.in/yaml.v3"
)

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

// DB carries a cleanup and declares no capability.
type DB struct{}

func NewDB(l *Logger) (*DB, func()) { return &DB{}, func() {} }

// App declares two capabilities and carries no cleanup.
type App struct{}

func NewApp(db *DB, node *yaml.Node) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

// Boot calls the lifecycle constructor. Every load this package takes during a
// run must resolve that call.
func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx, nil)

	return err
}
