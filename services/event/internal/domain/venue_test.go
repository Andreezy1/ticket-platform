package domain

import (
	"errors"
	"strings"
	"testing"
)

func validVenueParams() CreateVenueParams {
	return CreateVenueParams{
		Name:     "Екатеринбург Арена",
		Address:  "Екатеринбург, ул. Репина, 5",
		Timezone: "Asia/Yekaterinburg",
	}
}

func TestVenueParams_Validate(t *testing.T) {
	tests := []struct {
		name         string
		mutate       func(v *CreateVenueParams)
		wantErr      error
		wantName     string
		wantAddress  string
		wantTimezone string
	}{
		{
			name:    "valid params",
			mutate:  func(v *CreateVenueParams) {},
			wantErr: nil,
		},
		{
			name:     "valid trim space name",
			mutate:   func(v *CreateVenueParams) { v.Name = "   Екатеринбург Арена  " },
			wantErr:  nil,
			wantName: "Екатеринбург Арена",
		},
		{
			name:        "valid trim space address",
			mutate:      func(v *CreateVenueParams) { v.Address = "    Екатеринбург, ул. Репина, 5  " },
			wantErr:     nil,
			wantAddress: "Екатеринбург, ул. Репина, 5",
		},
		{
			name:         "valid trim space timezone",
			mutate:       func(v *CreateVenueParams) { v.Timezone = "  Asia/Yekaterinburg " },
			wantErr:      nil,
			wantTimezone: "Asia/Yekaterinburg",
		},
		{
			name:    "empty name",
			mutate:  func(v *CreateVenueParams) { v.Name = "  " },
			wantErr: ErrValidate,
		},
		{
			name:    "empty address",
			mutate:  func(v *CreateVenueParams) { v.Address = "  " },
			wantErr: ErrValidate,
		},
		{
			name:    "empty timezone",
			mutate:  func(v *CreateVenueParams) { v.Timezone = "  " },
			wantErr: ErrValidate,
		},
		{
			name:    "255-character name is accepted",
			mutate:  func(v *CreateVenueParams) { v.Name = strings.Repeat("ч", 255) },
			wantErr: nil,
		},
		{
			name:    "256-character name returns validation error",
			mutate:  func(v *CreateVenueParams) { v.Name = strings.Repeat("ч", 256) },
			wantErr: ErrValidate,
		},
		{
			name:    "64-character timezone is accepted",
			mutate:  func(v *CreateVenueParams) { v.Timezone = strings.Repeat("ч", 64) },
			wantErr: nil,
		},
		{
			name:    "65-character timezone returns validation error",
			mutate:  func(v *CreateVenueParams) { v.Timezone = strings.Repeat("ч", 65) },
			wantErr: ErrValidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			venue := validVenueParams()
			tt.mutate(&venue)

			err := venue.Validate()
			if tt.wantName != "" {
				if tt.wantName != venue.Name {
					t.Fatalf("name not normalized: got %q, want %q", venue.Name, tt.wantName)
				}
			}
			if tt.wantAddress != "" {
				if tt.wantAddress != venue.Address {
					t.Fatalf("address not normalized: got %q, want %q", venue.Address, tt.wantAddress)
				}
			}
			if tt.wantTimezone != "" {
				if tt.wantTimezone != venue.Timezone {
					t.Fatalf("timezone not normalized: got %q, want %q", venue.Timezone, tt.wantTimezone)
				}
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error %v is not wrapped with ErrValidate", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
