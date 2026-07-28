package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type User struct {
	Id              uuid.UUID
	Email           string
	PasswordHash    string
	Role            string
	IsActive        bool
	IsEmailVerified bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type UserParams struct {
	Email    string
	Password string
}

const (
	minPasswordLength = 10
	maxPasswordBytes  = 72
)

func (p *UserParams) Validate() error {
	p.Email = strings.TrimSpace(p.Email)
	p.Email = strings.ToLower(p.Email)

	if p.Email == "" {
		return fmt.Errorf("%w: email is required", ErrValidate)
	}
	addr, err := mail.ParseAddress(p.Email)
	if err != nil || addr.Address != p.Email {
		return fmt.Errorf("%w: invalid email format", ErrValidate)
	}
	if p.Password == "" {
		return fmt.Errorf("%w: password is required", ErrValidate)
	}
	if utf8.RuneCountInString(p.Password) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters long", ErrValidate, minPasswordLength)
	}
	if len(p.Password) > maxPasswordBytes {
		return fmt.Errorf("%w: password must be at most %d bytes long", ErrValidate, maxPasswordBytes)
	}

	return nil
}
