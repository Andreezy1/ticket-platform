package handler

import (
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/google/uuid"
)

type userRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *userRequest) toDomain() *domain.UserParams {
	return &domain.UserParams{
		Email:    r.Email,
		Password: r.Password,
	}
}

type createUserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func newCreateUserResponse(user *domain.User) createUserResponse {
	return createUserResponse{
		ID:        user.Id,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

type tokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type jWKSResponse struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

func newJWKSResponse(domains []domain.JWK) map[string][]jWKSResponse {
	jwks := make([]jWKSResponse, 0, len(domains))
	for _, d := range domains {
		jwks = append(jwks, jWKSResponse{
			Kty: d.Kty,
			Crv: d.Crv,
			X:   d.X,
			Kid: d.Kid,
			Use: d.Use,
			Alg: d.Alg,
		})
	}
	resp := map[string][]jWKSResponse{"keys": jwks}
	return resp
}
