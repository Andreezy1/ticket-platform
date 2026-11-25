package postgres

import (
	"context"

	"github.com/Andreezy1/ticket-platform/services/event/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type venueRepository struct {
	db *pgxpool.Pool
}

func CreateNewVenueRepository(db *pgxpool.Pool) *venueRepository {
	return &venueRepository{
		db: db,
	}
}

func (v *venueRepository) Create(ctx context.Context, venue domain.CreateVenueParams) (*domain.Venue, error) {
	query := `INSERT INTO venues (name, address, timezone)
			VALUES ($1, $2, $3)
			RETURNING id, name, address, timezone, created_at, updated_at
			`
	var venueReturn domain.Venue
	err := v.db.QueryRow(ctx,
		query,
		venue.Name,
		venue.Address,
		venue.Timezone,
	).Scan(&venueReturn.ID,
		&venueReturn.Name,
		&venueReturn.Address,
		&venueReturn.Timezone,
		&venueReturn.CreatedAt,
		&venueReturn.UpdatedAt,
	)

	if err != nil {
		return nil, mapError(err)
	}
	return &venueReturn, nil

}
