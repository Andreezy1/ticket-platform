package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/Andreezy1/ticket-platform/services/event/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func buildDSN(cfg *config.Config) string {
	query := url.Values{}
	query.Set("sslmode", cfg.DBSSLMode)

	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:     net.JoinHostPort(cfg.DBHost, cfg.DBPort),
		Path:     "/" + cfg.DBName,
		RawQuery: query.Encode(),
	}

	return u.String()
}

func NewPostgresConnect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := buildDSN(cfg)
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
