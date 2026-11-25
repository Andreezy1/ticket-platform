package service

import (
	"context"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
)

type VenueRepository interface {
	Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error)
}

type VenueService struct {
	vRepo VenueRepository
}

func CreateNewVenueService(vRepo VenueRepository) *VenueService {
	return &VenueService{
		vRepo: vRepo,
	}
}

func (v *VenueService) Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error) {
	err := venue.Validate()
	if err != nil {
		return nil, err
	}

	createVenue, err := v.vRepo.Create(ctx, venue)
	if err != nil {
		return nil, err
	}

	return createVenue, nil

}
