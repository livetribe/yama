package badwire

import "context"

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

// DB has no provider, so Google Wire cannot build App.
type DB struct{}

type App struct{}

func NewApp(db *DB) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}
