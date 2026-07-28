package domain

import "errors"

var (
	ErrValidate              = errors.New("validation failed")
	ErrNotFound              = errors.New("not found")
	ErrConflict              = errors.New("conflict")
	ErrRepositoryUnavailable = errors.New("database unavailable")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrAccountDisabled       = errors.New("account disabled")
	ErrInvalidRefreshToken   = errors.New("invalid refresh token")

	ErrTokenNotUsable = errors.New("token not usable")
)
