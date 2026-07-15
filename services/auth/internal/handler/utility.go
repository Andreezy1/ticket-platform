package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	status := statusFromErr(err)

	if status >= http.StatusInternalServerError {
		h.writeError(w, status, "internal server error")
		return
	}
	h.writeError(w, status, err.Error())
}

func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, body any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(body); err != nil {
		slog.Info("decode request error",
			slog.Any("error", err),
		)
		return err
	}
	return nil
}

func statusFromErr(err error) int {
	switch {
	// case errors.Is(err, domain.ErrNotFound):
	// 	return http.StatusNotFound
	// case errors.Is(err, domain.ErrConflict):
	// 	return http.StatusConflict
	// case errors.Is(err, domain.ErrValidate):
	// 	return http.StatusBadRequest
	// case errors.Is(err, domain.ErrQueueFull):
	// 	return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
