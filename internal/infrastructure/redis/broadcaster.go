package redis

import (
	"log/slog"

	"github.com/gomodule/redigo/redis"
)

type RedisBroadcaster struct {
	pool *redis.Pool
	log  *slog.Logger
}

func NewRedisBroadcaster(pool *redis.Pool, log *slog.Logger) *RedisBroadcaster {
	return &RedisBroadcaster{pool: pool, log: log}
}
