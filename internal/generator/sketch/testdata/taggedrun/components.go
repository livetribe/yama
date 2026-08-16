package taggedrun

import "context"

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

// DB is declared here and provided only under the special tag.
type DB struct{}

type App struct{}

func NewApp(db *DB) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}
