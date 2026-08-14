package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"

	"github.com/go-chi/chi/v5"
)

type UserService interface {
	Create(ctx context.Context, createdUser *domain.UserParams) (*domain.User, error)
	Login(ctx context.Context, user *domain.UserParams) (*domain.Tokens, error)
	Refresh(ctx context.Context, plain string) (*domain.Tokens, error)
	Logout(ctx context.Context, plain string) error
}

type TokenJWKS interface {
	JWKS() []domain.JWK
}

type Handler struct {
	userService UserService
	tokenJWKS   TokenJWKS
	ttl         time.Duration
}

func NewHandler(userService UserService,
	tokenJWKS TokenJWKS,
	ttl time.Duration,
) *Handler {
	return &Handler{
		userService: userService,
		tokenJWKS:   tokenJWKS,
		ttl:         ttl,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router, ipLimit func(http.Handler) http.Handler) {
	r.Post("/register", h.create)
	r.Get("/healthz", h.healthCheck)
	r.Get("/.well-known/jwks.json", h.jwks)
	r.With(ipLimit).Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	ctx := r.Context()
	log := logger.FromContext(ctx)
	if err := decodeJSON(w, r, &req); err != nil {
		log.Error("decode request error", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := h.userService.Create(ctx, req.toDomain())
	if err != nil {
		log.Info("create user error", slog.Any("error", err))
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newCreateUserResponse(user))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var u userRequest
	ctx := r.Context()
	log := logger.FromContext(ctx)
	if err := decodeJSON(w, r, &u); err != nil {
		log.Error("decode request error", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tokens, err := h.userService.Login(ctx, u.toDomain())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response := tokensResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.ttl.Seconds()),
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var refreshToken refreshRequest
	ctx := r.Context()
	log := logger.FromContext(ctx)
	if err := decodeJSON(w, r, &refreshToken); err != nil {
		log.Error("decode request error", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	newTokens, err := h.userService.Refresh(ctx, refreshToken.RefreshToken)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response := tokensResponse{
		AccessToken:  newTokens.AccessToken,
		RefreshToken: newTokens.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.ttl.Seconds()),
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var refreshToken refreshRequest
	ctx := r.Context()
	log := logger.FromContext(ctx)
	if err := decodeJSON(w, r, &refreshToken); err != nil {
		log.Error("decode request error", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.userService.Logout(ctx, refreshToken.RefreshToken); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) jwks(w http.ResponseWriter, r *http.Request) {
	keys := h.tokenJWKS.JWKS()
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, newJWKSResponse(keys))
}
