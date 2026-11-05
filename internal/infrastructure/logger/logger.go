package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
)

func MustSetupLogger(env string, cfg config.Log) *slog.Logger {
	log, err := newLogger(env, cfg)
	if err != nil {
		panic(err)
	}

	return log
}

func New(service, env string, cfg config.Log) *slog.Logger {
	return MustSetupLogger(env, cfg).With(slog.String("service", service))
}

func newLogger(env string, cfg config.Log) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var h slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		h = slog.NewJSONHandler(os.Stdout, opts)
	default:
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(h).With(slog.String("env", env)), nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
