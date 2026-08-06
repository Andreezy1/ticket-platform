package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-never-used"), bcrypt.DefaultCost)
	if err != nil {
		panic("failed create dummyHash" + err.Error())
	}
	dummyHash = h
}

type ManagerToken interface {
	IssueAccessToken(userID, role string) (string, error)
	GenerateRefreshToken() (plain string, hash string, err error)
	HashRefreshToken(plain string) string
}

type UserRepository interface {
	Create(ctx context.Context, user *domain.UserParams) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, ttl time.Duration) error
	FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, time.Time, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	RevokeByHash(ctx context.Context, tokenHash string) error
	Rotate(ctx context.Context, oldHash string, newHash string, ttl time.Duration) (userID uuid.UUID, err error)
}

type UserService struct {
	userRepo          UserRepository
	refreshTokenRepo  RefreshTokenRepository
	tokens            ManagerToken
	refreshTokenTTL   time.Duration
	refreshReuseGrace time.Duration
}

func NewUserService(userRepo UserRepository,
	refreshTokenRepo RefreshTokenRepository,
	tokens ManagerToken,
	refreshTokenTTL time.Duration,
	refreshReuseGrace time.Duration,
) *UserService {
	return &UserService{
		userRepo:          userRepo,
		refreshTokenRepo:  refreshTokenRepo,
		tokens:            tokens,
		refreshTokenTTL:   refreshTokenTTL,
		refreshReuseGrace: refreshReuseGrace,
	}
}

func (s *UserService) Create(ctx context.Context, createdUser *domain.UserParams) (*domain.User, error) {
	if err := createdUser.Validate(); err != nil {
		return nil, err
	}
	hashPass, err := hashPassword(createdUser.Password)
	if err != nil {
		return nil, err
	}
	var createdUserRepo = &domain.UserParams{
		Email:    createdUser.Email,
		Password: hashPass,
	}
	user, err := s.userRepo.Create(ctx, createdUserRepo)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Login(ctx context.Context, user *domain.UserParams) (*domain.Tokens, error) {
	log := logger.FromContext(ctx)
	if err := user.Validate(); err != nil {
		return nil, err
	}
	u, err := s.userRepo.FindByEmail(ctx, user.Email)
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(user.Password))
		return nil, domain.ErrInvalidCredentials
	default:
		log.Error("Repository unavailable", slog.Any("error", err))
		return nil, domain.ErrRepositoryUnavailable
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(user.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !u.IsActive {
		log.Info("Try enter in diactiveted account", slog.String("id", u.Id.String()))
		return nil, domain.ErrAccountDisabled
	}

	accessToken, err := s.tokens.IssueAccessToken(u.Id.String(), u.Role)
	if err != nil {
		log.Error("Failed to issue the token", slog.Any("error", err))
		return nil, err
	}
	refreshToken, refreshHash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.refreshTokenRepo.Create(ctx, u.Id, refreshHash, s.refreshTokenTTL); err != nil {
		return nil, err
	}
	return &domain.Tokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *UserService) Logout(ctx context.Context, plain string) error {
	if plain == "" {
		return domain.ErrValidate
	}
	hash := s.tokens.HashRefreshToken(plain)
	if err := s.refreshTokenRepo.RevokeByHash(ctx, hash); err != nil {
		return err
	}
	return nil
}

func (s *UserService) Refresh(ctx context.Context, plain string) (*domain.Tokens, error) {
	if plain == "" {
		return nil, domain.ErrInvalidRefreshToken
	}
	log := logger.FromContext(ctx)

	oldHash := s.tokens.HashRefreshToken(plain)

	newPlain, newHash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	userID, err := s.refreshTokenRepo.Rotate(ctx, oldHash, newHash, s.refreshTokenTTL)
	if errors.Is(err, domain.ErrTokenNotUsable) {
		refreshToken, nowDB, err := s.refreshTokenRepo.FindByHash(ctx, oldHash)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidRefreshToken
		}
		if err != nil {
			return nil, err
		}
		switch {
		case refreshToken.WasRotated() && refreshToken.IsExpired(nowDB):
			log.Warn("token rotated and expired", slog.String("userID", refreshToken.UserID.String()))
			return nil, domain.ErrInvalidRefreshToken
		case refreshToken.WasRotated() && refreshToken.IsRevokedInReuseGraceWindow(nowDB, s.refreshReuseGrace):
			return nil, domain.ErrInvalidRefreshToken
		case refreshToken.WasRotated() && !refreshToken.IsRevokedInReuseGraceWindow(nowDB, s.refreshReuseGrace):
			attrs := []any{
				slog.String("userID", refreshToken.UserID.String()),
				slog.String("refreshTokenID", refreshToken.ID.String()),
			}
			if refreshToken.RevokedAt != nil {
				attrs = append(attrs, slog.Time("revokedAtyf", *refreshToken.RevokedAt))
			}
			log.Warn("token reused", attrs...)
			if err := s.refreshTokenRepo.RevokeAllForUser(ctx, refreshToken.UserID); err != nil {
				log.Error("failed to revoke user sessions", slog.Any("error", err))
				return nil, domain.ErrInvalidRefreshToken
			}
			return nil, domain.ErrInvalidRefreshToken
		case !refreshToken.WasRotated() && refreshToken.IsRevoked():
			return nil, domain.ErrInvalidRefreshToken
		case refreshToken.IsExpired(nowDB):
			return nil, domain.ErrInvalidRefreshToken
		default:
			log.Error("unclassified refresh token state",
				slog.String("userID", refreshToken.UserID.String()),
				slog.String("refreshTokenID", refreshToken.ID.String()),
				slog.Bool("rotated", refreshToken.WasRotated()),
				slog.Bool("revoked", refreshToken.IsRevoked()),
				slog.Bool("expired", refreshToken.IsExpired(nowDB)),
			)
			return nil, domain.ErrInvalidRefreshToken
		}

	}
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		if err := s.refreshTokenRepo.RevokeByHash(ctx, newHash); err != nil {
			return nil, err
		}
		return nil, domain.ErrAccountDisabled
	}
	accessToken, err := s.tokens.IssueAccessToken(userID.String(), user.Role)
	if err != nil {
		return nil, err
	}
	return &domain.Tokens{AccessToken: accessToken, RefreshToken: newPlain}, nil
}
