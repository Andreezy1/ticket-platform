package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Venue struct {
	ID        uuid.UUID
	Name      string
	Address   string
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateVenueParams struct {
	Name     string
	Address  string
	Timezone string
}

const (
	maxTimezoneCharacters = 64
	maxNameCharacters     = 255
)

func (p *CreateVenueParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Address = strings.TrimSpace(p.Address)
	p.Timezone = strings.TrimSpace(p.Timezone)

	if p.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidate)
	}
	if p.Address == "" {
		return fmt.Errorf("%w: address is required", ErrValidate)
	}
	if p.Timezone == "" {
		return fmt.Errorf("%w: timezone is required", ErrValidate)
	}

	if utf8.RuneCountInString(p.Name) > maxNameCharacters {
		return fmt.Errorf("%w: name must be at most %d characters long", ErrValidate, maxNameCharacters)
	}
	if utf8.RuneCountInString(p.Timezone) > maxTimezoneCharacters {
		return fmt.Errorf("%w: timezone must be at most %d characters long", ErrValidate, maxTimezoneCharacters)
	}

	return nil
}
