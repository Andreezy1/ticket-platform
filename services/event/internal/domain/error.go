package domain

import "errors"

var (
	ErrValidate              = errors.New("validation failed")
	ErrNotFound              = errors.New("not found")
	ErrConflict              = errors.New("conflict")
	ErrRepositoryUnavailable = errors.New("database unavailable")
)
