package valuerun

import "context"

type Config struct{ Env string }

type App struct{}

func NewApp(c Config) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}
