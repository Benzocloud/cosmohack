package repository

import "github.com/Benzocloud/cosmohack/backend/internal/domain"

// ErrNotFound indicates that a requested aggregate does not exist.
var (
	ErrNotFound = domain.ErrNotFound
	ErrConflict = domain.ErrConflict
	ErrBadState = domain.ErrBadState
)
