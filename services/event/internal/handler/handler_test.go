package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
	"github.com/google/uuid"
)

type createVenueCall struct {
	name     string
	address  string
	timezone string
}

type fakeVenueService struct {
	createVenue      *domain.Venue
	createErr        error
	createVenueCalls []createVenueCall
}

func (v *fakeVenueService) Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error) {
	v.createVenueCalls = append(v.createVenueCalls, createVenueCall{name: venue.Name, address: venue.Address, timezone: venue.Timezone})
	return v.createVenue, v.createErr
}

func newJSONRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ---------------------------------------------------------------------
// create
// ---------------------------------------------------------------------

func TestHandler_Create_Success(t *testing.T) {
	svc := &fakeVenueService{
		createVenue: &domain.Venue{
			ID:        uuid.New(),
			Name:      "Екатеринбург Арена",
			Address:   "Екатеринбург, ул. Репина, 5",
			Timezone:  "Asia/Yekaterinburg",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now().Add(10 * time.Hour),
		},
	}
	h := NewHandler(svc)

	req := newJSONRequest(t, http.MethodPost, "/venues", `{"name":"Екатеринбург Арена","address":"Екатеринбург, ул. Репина, 5","timezone":"Asia/Yekaterinburg"}`)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty body on 201")
	}

	if len(svc.createVenueCalls) != 1 {
		t.Fatalf("service not calls")
	}
	if svc.createVenueCalls[0].name != svc.createVenue.Name {
		t.Errorf("service.Create called with %s, want name %s", svc.createVenueCalls[0].name, svc.createVenue.Name)
	}
	if svc.createVenueCalls[0].address != svc.createVenue.Address {
		t.Errorf("service.Create called with %s, want address %s", svc.createVenueCalls[0].address, svc.createVenue.Address)
	}
	if svc.createVenueCalls[0].timezone != svc.createVenue.Timezone {
		t.Errorf("service.Create called with %s, want timezone %s", svc.createVenueCalls[0].timezone, svc.createVenue.Timezone)
	}

	var resp createVenuesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.ID != svc.createVenue.ID {
		t.Errorf("resp.ID = %q, want %q", resp.ID, svc.createVenue.ID)
	}
	if resp.Name != svc.createVenue.Name {
		t.Errorf("resp.Name = %q, want %q", resp.Name, svc.createVenue.Name)
	}
	if resp.Address != svc.createVenue.Address {
		t.Errorf("resp.Address = %q, want %q", resp.Address, svc.createVenue.Address)
	}
	if resp.Timezone != svc.createVenue.Timezone {
		t.Errorf("resp.Timezone = %q, want %q", resp.Timezone, svc.createVenue.Timezone)
	}
	if !resp.CreatedAt.Equal(svc.createVenue.CreatedAt) {
		t.Errorf("resp.CreatedAT = %q, want %q", resp.CreatedAt, svc.createVenue.CreatedAt)
	}
	if !resp.UpdatedAt.Equal(svc.createVenue.UpdatedAt) {
		t.Errorf("resp.UpdatedAt = %q, want %q", resp.UpdatedAt, svc.createVenue.UpdatedAt)
	}
}

func TestHandler_Create_IncorectBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid json",
			body: `{"name":}`,
		},
		{
			name: "two object in 1 HTTP request",
			body: `{"name":"Арена","address":"ул. Репина, 5","timezone":"Asia/Yekaterinburg"} {}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeVenueService{
				createVenue: &domain.Venue{},
			}
			h := NewHandler(svc)

			req := newJSONRequest(t, http.MethodPost, "/venues", tt.body)
			rec := httptest.NewRecorder()

			h.create(rec, req)

			if len(svc.createVenueCalls) != 0 {
				t.Fatalf("service calls")
			}

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHandler_Create_ValidateError(t *testing.T) {
	svc := &fakeVenueService{
		createErr: domain.ErrValidate,
	}
	h := NewHandler(svc)

	req := newJSONRequest(t, http.MethodPost, "/venues", `{}`)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	if len(svc.createVenueCalls) != 1 {
		t.Fatalf("service call %d, want %d", len(svc.createVenueCalls), 1)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

}
