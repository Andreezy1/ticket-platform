package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	TokenHash         string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	RevokedAt         *time.Time
	ReplacedByTokenID *uuid.UUID
}

func (t *RefreshToken) IsRevokedInReuseGraceWindow(nowDB time.Time, refreshReuseGraceTTL time.Duration) bool {
	if t.RevokedAt == nil {
		return false
	}
	deadline := t.RevokedAt.Add(refreshReuseGraceTTL)
	return nowDB.Before(deadline)
}

func (t *RefreshToken) IsExpired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

func (t *RefreshToken) WasRotated() bool {
	return t.ReplacedByTokenID != nil
}

func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

type JWK struct {
	Kty string
	Crv string
	X   string
	Kid string
	Use string
	Alg string
}
