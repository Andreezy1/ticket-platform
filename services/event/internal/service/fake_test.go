package service_test

import (
	"context"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
)

type createVenueCall struct {
	name     string
	address  string
	timezone string
	context  context.Context
}

type fakeVenueRepo struct {
	createVenue      *domain.Venue
	createErr        error
	createVenueCalls []createVenueCall
}

func (v *fakeVenueRepo) Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error) {
	v.createVenueCalls = append(v.createVenueCalls, createVenueCall{name: venue.Name, address: venue.Address, timezone: venue.Timezone, context: ctx})
	return v.createVenue, v.createErr
}
