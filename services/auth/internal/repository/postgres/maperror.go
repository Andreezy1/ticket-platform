package postgres

import (
	"errors"
	"fmt"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation: // 23505 → 409
			return fmt.Errorf("%w: this value already exists", domain.ErrConflict)
		case pgerrcode.CheckViolation: // 23514 → 400
			return fmt.Errorf("%w: invalid input", domain.ErrValidate)
		}
	}
	return err
}
