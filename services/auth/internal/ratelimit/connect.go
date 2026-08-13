package ratelimit

import (
	"context"
	"log/slog"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisConnect(ctx context.Context, cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddress,
		DialTimeout:  cfg.RedisDialTimeout,
		ReadTimeout:  cfg.RedisReadTimeout,
		WriteTimeout: cfg.RedisWriteTimeout,
		PoolTimeout:  cfg.RedisPoolTimeout,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("ping redis", slog.Any("error", err))
	}
	return rdb
}
