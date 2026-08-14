package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const refreshEntropyBytes = 32

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type TokenService struct {
	priv            ed25519.PrivateKey
	pub             ed25519.PublicKey
	kid             string
	issuer          string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewTokenService(priv ed25519.PrivateKey,
	pub ed25519.PublicKey,
	issuer string,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
) *TokenService {
	return &TokenService{
		priv:            priv,
		pub:             pub,
		kid:             computeKID(pub),
		issuer:          issuer,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (t *TokenService) IssueAccessToken(userID, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTokenTTL)),
			ID:        uuid.NewString(),
			Issuer:    t.issuer,
		},
	}

	jt := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	jt.Header["kid"] = t.kid

	signed, err := jt.SignedString(t.priv)
	if err != nil {
		return "", fmt.Errorf("signed token error: %w", err)
	}
	return signed, nil
}

func (t *TokenService) GenerateRefreshToken() (plain string, hash string, err error) {
	buf := make([]byte, refreshEntropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("token: generate refresh: %w", err)
	}

	plain = base64.RawURLEncoding.EncodeToString(buf)
	hash = t.HashRefreshToken(plain)
	return plain, hash, nil
}

func (t *TokenService) HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (t *TokenService) JWKS() []domain.JWK {
	x := base64.RawURLEncoding.EncodeToString(t.pub)
	jwks := domain.JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   x,
		Kid: t.kid,
		Use: "sig",
		Alg: "EdDSA",
	}
	keys := make([]domain.JWK, 0, 1)
	keys = append(keys, jwks)
	return keys
}
