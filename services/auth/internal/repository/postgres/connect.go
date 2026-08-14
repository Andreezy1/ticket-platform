package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresConnect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	hostPort := net.JoinHostPort(cfg.DBHost, cfg.DBPort)
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=%s",
		url.QueryEscape(cfg.DBUser),
		url.QueryEscape(cfg.DBPassword),
		hostPort,
		cfg.DBName,
		cfg.DBSSLMode,
	)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}
