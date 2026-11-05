package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/app"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/logger"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := config.MustLoad[config.Config]()
	log := logger.New("gl1tch-ssh", cfg.Env, cfg.Logger())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("failed to create app", slog.Any("error", err))

		return 1
	}

	if err = application.Run(ctx); err != nil {
		log.Error("application stopped with error", slog.Any("error", err))

		return 1
	}

	log.Info("gl1tch-ssh stopped")

	return 0
}
