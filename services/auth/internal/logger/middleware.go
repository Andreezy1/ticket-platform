package logger

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())

		l := slog.Default()
		if reqID != "" {
			l = l.With(slog.String("request_id", reqID))
		}

		ctx := WithFields(r.Context(), l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
