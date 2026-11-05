package app

import (
	"context"
	"log/slog"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/app/di"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
)

type App struct {
	di *di.Container
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	container, err := di.New(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	return &App{
		di: container,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.di.Close()

	return a.di.Serve(ctx)
}
