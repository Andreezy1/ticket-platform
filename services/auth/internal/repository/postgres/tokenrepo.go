package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		db: db,
	}
}

const refreshTokenColumns = `id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by_token_id`

func (t *RefreshTokenRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash string, ttl time.Duration) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
			VALUES ($1, $2, now() + $3)`

	_, err := t.db.Exec(ctx,
		query,
		userID,
		tokenHash,
		ttl.String(),
	)
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (t *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, time.Time, error) {
	query := `SELECT ` + refreshTokenColumns + `, now() AS db_now` +
		` FROM refresh_tokens
			WHERE token_hash = $1`
	var token domain.RefreshToken
	var dbNow time.Time
	err := t.db.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.RevokedAt,
		&token.ReplacedByTokenID,
		&dbNow,
	)
	if err != nil {
		return nil, time.Time{}, mapError(err)
	}
	return &token, dbNow, nil
}

func (t *RefreshTokenRepository) Rotate(
	ctx context.Context,
	oldHash, newHash string,
	ttl time.Duration,
) (userID uuid.UUID, err error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, mapError(err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	captureQ := `
		UPDATE refresh_tokens
		SET revoked_at = clock_timestamp()
		WHERE token_hash = $1
		AND revoked_at IS NULL
		AND expires_at > now()
		RETURNING id, user_id`

	var oldID, ownerID uuid.UUID
	err = tx.QueryRow(ctx, captureQ, oldHash).Scan(&oldID, &ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrTokenNotUsable
	}
	if err != nil {
		return uuid.Nil, mapError(err)
	}

	insertQ := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + $3)
		RETURNING id`

	var newID uuid.UUID
	if err := tx.QueryRow(ctx,
		insertQ,
		ownerID,
		newHash,
		ttl.String()).Scan(&newID); err != nil {
		return uuid.Nil, mapError(err)
	}

	linkQ := `UPDATE refresh_tokens
			  SET replaced_by_token_id = $2
			  WHERE id = $1`
	if _, err := tx.Exec(ctx, linkQ, oldID, newID); err != nil {
		return uuid.Nil, mapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, mapError(err)
	}

	return ownerID, nil
}

func (t *RefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string) error {
	query := `
			UPDATE refresh_tokens
			SET revoked_at = now()
			WHERE token_hash = $1
			AND revoked_at IS NULL`

	_, err := t.db.Exec(ctx, query, tokenHash)
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (t *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	query := `
			UPDATE refresh_tokens
			SET revoked_at = now()
			WHERE user_id = $1
			AND revoked_at IS NULL`

	_, err := t.db.Exec(ctx, query, userID)
	if err != nil {
		return mapError(err)
	}
	return nil
}
