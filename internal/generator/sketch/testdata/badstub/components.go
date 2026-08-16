package badstub

import "context"

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

type App struct{}

func NewApp(l *Logger) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }

func (a *App) Stop(_ context.Context) {}
