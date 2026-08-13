package ratelimit

import (
	"context"
	_ "embed"
	"log/slog"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

//go:embed sliding_window.lua
var slidingWindowScript string

var script = redis.NewScript(slidingWindowScript)

type Limiter struct {
	client  redis.Cmdable
	timeout time.Duration
}

func NewLimiter(client redis.Cmdable, timeout time.Duration) *Limiter {
	return &Limiter{
		client:  client,
		timeout: timeout,
	}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	log := logger.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	member := uuid.NewString()

	res, err := script.Run(
		ctx,
		l.client,
		[]string{key},
		window.Milliseconds(),
		limit, member,
	).Int64()
	if err != nil {
		log.Error("ratelimit: redis call failed, failing open", slog.Any("error", err), slog.String("key", key))
		return true
	}

	return res == 1
}

func (l *Limiter) Reset(ctx context.Context, key string) {
	log := logger.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	if err := l.client.Del(ctx, key).Err(); err != nil {
		log.Error("ratelimit: reset failed, ignoring", slog.Any("error", err), slog.String("key", key))
	}
}
