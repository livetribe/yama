package setrun

import (
	"context"

	"l7e.io/yama/v2/internal/generator/sketch/testdata/setother"
)

type App struct{}

func NewApp(t *setother.Thing) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}
