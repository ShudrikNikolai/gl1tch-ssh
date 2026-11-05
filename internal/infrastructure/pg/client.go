package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pingTimeout = 5 * time.Second

type Client struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Database) (*Client, error) {
	cfgPgx, err := pgxpool.ParseConfig(cfg.PgDsn)
	if err != nil {
		return nil, fmt.Errorf("parse pg dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfgPgx)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping pg: %w", err)
	}

	return &Client{pool: pool}, nil
}

func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

func (c *Client) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func (c *Client) Close() {
	c.pool.Close()
}
