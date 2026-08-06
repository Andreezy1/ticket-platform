package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRefreshToken_IsRevokedInReuseGraceWindow(t *testing.T) {
	base := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	grace := 15 * time.Second

	tests := []struct {
		name      string
		revokedAt *time.Time
		now       time.Time
		grace     time.Duration
		want      bool
	}{
		{
			name:      "not revoked returns false",
			revokedAt: nil,
			now:       base.Add(5 * time.Second),
			grace:     grace,
			want:      false,
		},
		{
			name:      "zero grace period means no window at all",
			revokedAt: &base,
			now:       base,
			grace:     0,
			want:      false,
		},
		{
			name:      "zero grace period rejects even the same nanosecond",
			revokedAt: &base,
			now:       base.Add(time.Nanosecond),
			grace:     0,
			want:      false,
		},
		{
			name:      "now equal to deadline is outside grace window",
			revokedAt: &base,
			now:       base.Add(grace),
			grace:     grace,
			want:      false,
		},
		{
			name:      "now after deadline is outside grace window",
			revokedAt: &base,
			now:       base.Add(20 * time.Second),
			grace:     grace,
			want:      false,
		},
		{
			name:      "now just before deadline is within grace window",
			revokedAt: &base,
			now:       base.Add(grace - time.Nanosecond),
			grace:     grace,
			want:      true,
		},
		{
			name:      "revoked_at in the future is still treated as within grace window",
			revokedAt: &base,
			now:       base.Add(-time.Hour),
			grace:     grace,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := RefreshToken{RevokedAt: tt.revokedAt}

			got := token.IsRevokedInReuseGraceWindow(tt.now, tt.grace)

			if got != tt.want {
				t.Errorf("IsRevokedInReuseGraceWindow() = %v, want %v (now=%v, revokedAt=%v, grace=%v)",
					got, tt.want, tt.now, tt.revokedAt, tt.grace)
			}
		})
	}
}

func TestRefreshToken_IsExpired(t *testing.T) {
	expiresAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "now before expiry is not expired",
			expiresAt: expiresAt,
			now:       expiresAt.Add(-time.Second),
			want:      false,
		},
		{
			name:      "now equal to expiry is not expired",
			expiresAt: expiresAt,
			now:       expiresAt,
			want:      false,
		},
		{
			name:      "now after expiry is expired",
			expiresAt: expiresAt,
			now:       expiresAt.Add(time.Second),
			want:      true,
		},
		{
			name: "zero-value ExpiresAt (unset field) is treated as expired",
			now:  expiresAt.Add(time.Second),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := RefreshToken{ExpiresAt: tt.expiresAt}

			got := token.IsExpired(tt.now)

			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v (now=%v, expiresAt=%v)",
					got, tt.want, tt.now, expiresAt)
			}
		})
	}
}

func TestRefreshToken_IsRevoked(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		revokedAt *time.Time
		want      bool
	}{
		{
			name:      "nil RevokedAt is not revoked",
			revokedAt: nil,
			want:      false,
		},
		{
			name:      "set RevokedAt is revoked",
			revokedAt: &now,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := RefreshToken{RevokedAt: tt.revokedAt}

			if got := token.IsRevoked(); got != tt.want {
				t.Errorf("IsRevoked() = %v, want %v (revokedAt=%v)", got, tt.want, tt.revokedAt)
			}
		})
	}
}

func TestRefreshToken_WasRotated(t *testing.T) {
	newID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []struct {
		name              string
		replacedByTokenID *uuid.UUID
		want              bool
	}{
		{
			name:              "nil ReplacedByTokenID is not rotated",
			replacedByTokenID: nil,
			want:              false,
		},
		{
			name:              "set ReplacedByTokenID is rotated",
			replacedByTokenID: &newID,
			want:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := RefreshToken{ReplacedByTokenID: tt.replacedByTokenID}

			if got := token.WasRotated(); got != tt.want {
				t.Errorf("WasRotated() = %v, want %v (replacedByTokenID=%v)",
					got, tt.want, tt.replacedByTokenID)
			}
		})
	}
}

func TestRefreshToken_StateSemantics(t *testing.T) {
	now := time.Now()
	replacementID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []struct {
		name           string
		revokedAt      *time.Time
		replacedByID   *uuid.UUID
		wantIsRevoked  bool
		wantWasRotated bool
		meaning        string
	}{
		{
			name:           "both fields nil means alive token",
			revokedAt:      nil,
			replacedByID:   nil,
			wantIsRevoked:  false,
			wantWasRotated: false,
			meaning:        "живой токен",
		},
		{
			name:           "revoked without replacement means logout",
			revokedAt:      &now,
			replacedByID:   nil,
			wantIsRevoked:  true,
			wantWasRotated: false,
			meaning:        "logout",
		},
		{
			name:           "revoked with replacement means rotated",
			revokedAt:      &now,
			replacedByID:   &replacementID,
			wantIsRevoked:  true,
			wantWasRotated: true,
			meaning:        "ротирован",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := RefreshToken{
				RevokedAt:         tt.revokedAt,
				ReplacedByTokenID: tt.replacedByID,
			}

			gotRevoked := token.IsRevoked()
			gotRotated := token.WasRotated()

			if gotRevoked != tt.wantIsRevoked || gotRotated != tt.wantWasRotated {
				t.Errorf("состояние %q: IsRevoked()=%v WasRotated()=%v, want IsRevoked()=%v WasRotated()=%v",
					tt.meaning, gotRevoked, gotRotated, tt.wantIsRevoked, tt.wantWasRotated)
			}
		})
	}
}
