package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/event/internal/logger"
	"github.com/go-chi/chi/v5"
)

type VenuesService interface {
	Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error)
}

type Handler struct {
	venuesService VenuesService
}

func NewHandler(venuesService VenuesService) *Handler {
	return &Handler{venuesService: venuesService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/healthz", h.healthCheck)
	r.Post("/venues", h.create)

}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req requestVenuesCreate
	ctx := r.Context()
	log := logger.FromContext(ctx)

	if err := decodeJSON(w, r, &req); err != nil {
		log.Error("decode request error", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	venues, err := h.venuesService.Create(ctx, req.toDomain())
	if err != nil {
		log.Info("create venues error", slog.Any("error", err))
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newCreateVenuesResponse(venues))
}
