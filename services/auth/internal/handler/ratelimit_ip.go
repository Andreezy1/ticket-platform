package handler

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/ratelimit"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) bool
}

func NewIPRateLimit(
	l RateLimiter,
	limit int,
	window time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logger.FromContext(ctx)

			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				log.Warn("ratelimit: unexpected RemoteAddr format, failing open",
					slog.String("remote_addr", r.RemoteAddr),
					slog.Any("error", err))
				next.ServeHTTP(w, r)
				return
			}

			key, ok := ratelimit.LoginIPKey(host)
			if !ok {
				log.Error("ratelimit: unparseable ip, failing open",
					slog.String("remote_addr", r.RemoteAddr))
				next.ServeHTTP(w, r)
				return
			}

			if !l.Allow(ctx, key, limit, window) {
				writeServiceError(w, domain.ErrTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
