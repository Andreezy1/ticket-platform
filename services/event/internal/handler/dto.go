package handler

import (
	"time"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
	"github.com/google/uuid"
)

type requestVenuesCreate struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Timezone string `json:"timezone"`
}

func (r *requestVenuesCreate) toDomain() domain.CreateVenueParams {
	return domain.CreateVenueParams{
		Name:     r.Name,
		Address:  r.Address,
		Timezone: r.Timezone,
	}
}

type createVenuesResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newCreateVenuesResponse(venues *domain.Venue) createVenuesResponse {
	return createVenuesResponse{
		ID:        venues.ID,
		Name:      venues.Name,
		Address:   venues.Address,
		Timezone:  venues.Timezone,
		CreatedAt: venues.CreatedAt,
		UpdatedAt: venues.UpdatedAt,
	}
}
