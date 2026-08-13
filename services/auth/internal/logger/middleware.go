package logger

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		ctx := r.Context()
		if reqID != "" {
			ctx = WithFields(r.Context(), slog.String("request_id", reqID))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
