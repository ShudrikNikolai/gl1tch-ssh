package di

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/pg"
	chatsRepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	chatsSrv "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	messagesRepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository"
	messagesSrv "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	usersRepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository"
	usersSrv "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/transport"
	"github.com/gomodule/redigo/redis"
)

type Container struct {
	Cfg config.Config
	Log *slog.Logger

	Pgx   *pg.Client
	Redis *redis.Pool
	SSH   *transport.Server

	ChatsRepo    chatsRepo.Repository
	ChatsSrv     chatsSrv.Service
	MessagesRepo messagesRepo.Repository
	MessagesSrv  messagesSrv.Service
	Broadcaster  messagesSrv.Broadcaster
	UsersRepo    usersRepo.Repository
	UsersSrv     usersSrv.Service
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*Container, error) {
	c := &Container{
		Cfg: cfg,
		Log: log,
	}

	steps := []struct {
		name string
		run  func() error
	}{
		{"storage", func() error { return c.initStorage(ctx) }},
		{"migrate", func() error { return c.initMigration(ctx) }},
		{"redis", c.initRedis},
		{"repo", func() error { return c.initRepo(ctx) }},
		{"services", func() error { return c.initServices(ctx) }},
		{"servers", c.initServers},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			log.Error("failed to init "+s.name, slog.Any("error", err))
			c.Close()

			return nil, err
		}
	}

	return c, nil
}

// Close освобождает ресурсы. Серверы останавливает Serve.
func (c *Container) Close() {
	if c.SSH != nil {
		_ = c.SSH.Close()
	}
	if c.Redis != nil {
		_ = c.Redis.Close()
	}
	if c.Pgx != nil {
		c.Pgx.Close()
	}
}

func (c *Container) Serve(ctx context.Context) error {
	if c.SSH == nil {
		return errors.New("ssh server is not initialized")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.SSH.Serve()
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errCh:
		if serveErr != nil {
			c.Log.Error("ssh server stopped unexpectedly", slog.Any("error", serveErr))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.Cfg.ShutdownTimeout)
	defer cancel()

	if err := c.SSH.Shutdown(shutdownCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.Log.Warn("ssh shutdown timed out, forcing close")
			_ = c.SSH.Close()
		} else if serveErr == nil {
			serveErr = err
		}
	}

	return serveErr
}
