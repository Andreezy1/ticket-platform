package postgres

import (
	"context"
	"log/slog"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (u *UserRepository) Create(ctx context.Context, user *domain.UserParams) (*domain.User, error) {
	log := logger.FromContext(ctx)
	query := `INSERT INTO users(email, password_hash) 
			VALUES ($1, $2) 
			RETURNING id, email, created_at`

	var createdUser domain.User
	err := u.db.QueryRow(ctx, query, user.Email, user.Password).Scan(
		&createdUser.Id,
		&createdUser.Email,
		&createdUser.CreatedAt,
	)
	if err != nil {
		log.Error("create user error", slog.Any("error", err))
		return nil, mapError(err)
	}
	return &createdUser, nil
}

func (u *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, role, is_active, is_email_verified, created_at, updated_at
			FROM users
			WHERE  lower(email) = lower($1)`
	var User domain.User
	err := u.db.QueryRow(ctx, query, email).Scan(
		&User.Id,
		&User.Email,
		&User.PasswordHash,
		&User.Role,
		&User.IsActive,
		&User.IsEmailVerified,
		&User.CreatedAt,
		&User.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &User, nil
}

func (u *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, password_hash, role, is_active, is_email_verified, created_at, updated_at
			FROM users
			WHERE  id = $1`
	var User domain.User
	err := u.db.QueryRow(ctx, query, userID).Scan(
		&User.Id,
		&User.Email,
		&User.PasswordHash,
		&User.Role,
		&User.IsActive,
		&User.IsEmailVerified,
		&User.CreatedAt,
		&User.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &User, nil
}
