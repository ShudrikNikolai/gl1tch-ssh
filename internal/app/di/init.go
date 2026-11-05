package di

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/pg"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/pg/migrations"
	redisinfra "github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/redis"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/transport"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func (c *Container) initStorage(ctx context.Context) error {
	pgx, err := pg.New(ctx, c.Cfg.Database)
	if err != nil {
		return err
	}
	c.Pgx = pgx

	return nil
}

func (c *Container) initMigration(ctx context.Context) error {
	db, err := sql.Open("pgx", c.Cfg.Database.PgDsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(10, 30))
	if err != nil {
		return fmt.Errorf("create migration locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations.FS,
		goose.WithSessionLocker(locker),
		goose.WithTableName("schema_migrations"),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	c.Log.Info("migrations applied", "count", len(results))

	return nil
}

func (c *Container) initRedis() error {
	pool := redisinfra.NewPool(c.Cfg.RedisAddr)

	conn := pool.Get()
	defer func() { _ = conn.Close() }()

	if _, err := conn.Do("PING"); err != nil {
		_ = pool.Close()

		return fmt.Errorf("ping redis: %w", err)
	}

	c.Redis = pool

	return nil
}

func (c *Container) initRepo(_ context.Context) error {
	return nil
}

func (c *Container) initServices(_ context.Context) error {
	return nil
}

func (c *Container) initServers() error {
	srv, err := transport.NewServer(c.Cfg, c.Log)
	if err != nil {
		return err
	}
	c.SSH = srv

	return nil
}
