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
	ErrTooManyRequests       = errors.New("too many requests")

	ErrTokenNotUsable = errors.New("token not usable")
)
