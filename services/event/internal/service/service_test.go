package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/event/internal/service"
	"github.com/google/uuid"
)

var repoErr = errors.New("write failed")

var (
	testVenueID = uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	timeDB      = time.Date(2026, 01, 01, 12, 0, 0, 0, time.UTC)
)

type stand struct {
	svc       *service.VenueService
	venueRepo *fakeVenueRepo
}

func newStand(
	t *testing.T,
) *stand {
	t.Helper()
	s := &stand{
		venueRepo: &fakeVenueRepo{},
	}
	s.svc = service.CreateNewVenueService(
		s.venueRepo,
	)
	return s
}

func validVenueParams() *domain.CreateVenueParams {
	return &domain.CreateVenueParams{
		Name:     "Екатеринбург Арена",
		Address:  "Екатеринбург, ул. Репина, 5",
		Timezone: "Asia/Yekaterinburg",
	}
}

func createVenueValid() *domain.Venue {
	return &domain.Venue{
		ID:        testVenueID,
		Name:      "Екатеринбург Арена",
		Address:   "Екатеринбург, ул. Репина, 5",
		Timezone:  "Asia/Yekaterinburg",
		CreatedAt: timeDB,
		UpdatedAt: timeDB.Add(10 * time.Hour),
	}
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name         string
		mutate       func(s *stand)
		mutateParams func(p *domain.CreateVenueParams)
		wantErr      error
		wantParams   *domain.CreateVenueParams
		wantCalls    int
	}{
		{
			name: "succefull create venue",
			mutate: func(s *stand) {
				s.venueRepo.createErr = nil
				s.venueRepo.createVenue = createVenueValid()
			},
			mutateParams: func(p *domain.CreateVenueParams) {
				p.Name = "  Екатеринбург Арена  "
				p.Address = "  Екатеринбург, ул. Репина, 5  "
				p.Timezone = " Asia/Yekaterinburg "
			},
			wantErr:    nil,
			wantParams: validVenueParams(),
			wantCalls:  1,
		},
		{
			name: "empty name",
			mutateParams: func(p *domain.CreateVenueParams) {
				p.Name = "  "
			},
			wantErr:    domain.ErrValidate,
			wantParams: nil,
			wantCalls:  0,
		},
		{
			name: "error from repository",
			mutate: func(s *stand) {
				s.venueRepo.createErr = repoErr
			},
			wantErr:    repoErr,
			wantParams: nil,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t)
			if tt.mutate != nil {
				tt.mutate(s)
			}
			venue := validVenueParams()
			if tt.mutateParams != nil {
				tt.mutateParams(venue)
			}

			createVenue, err := s.svc.Create(t.Context(), *venue)

			if len(s.venueRepo.createVenueCalls) != tt.wantCalls {
				t.Fatalf("Create() called with %d args, want %d", len(s.venueRepo.createVenueCalls), tt.wantCalls)
			}
			if tt.wantCalls == 1 {
				if s.venueRepo.createVenueCalls[0].context != t.Context() {
					t.Errorf("Crate() create called args with %s, want %s", s.venueRepo.createVenueCalls[0].context, t.Context())
				}
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
				}
				if createVenue != nil {
					t.Fatalf("Create() venue want nil, got %s", createVenue)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if createVenue == nil {
				t.Fatalf("Create() venue got nil, want %s", createVenueValid())
			}
			if *createVenue != *createVenueValid() {
				t.Errorf("Create() create Venue not equal: %s, want %s", createVenue, createVenueValid())
			}
			if s.venueRepo.createVenueCalls[0].name != tt.wantParams.Name {
				t.Errorf("Crate() create called args with %s, want %s", s.venueRepo.createVenueCalls[0].name, tt.wantParams.Name)
			}
			if s.venueRepo.createVenueCalls[0].address != tt.wantParams.Address {
				t.Errorf("Crate() create called args with %s, want %s", s.venueRepo.createVenueCalls[0].address, tt.wantParams.Address)
			}
			if s.venueRepo.createVenueCalls[0].timezone != tt.wantParams.Timezone {
				t.Errorf("Crate() create called args with %s, want %s", s.venueRepo.createVenueCalls[0].timezone, tt.wantParams.Timezone)
			}

		})
	}
}
