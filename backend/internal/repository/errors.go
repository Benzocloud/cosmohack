package repository

import "github.com/Benzocloud/cosmohack/backend/internal/domain"

// ErrNotFound означает, что запрошенный агрегат не существует.
var (
	ErrNotFound = domain.ErrNotFound
	ErrConflict = domain.ErrConflict
	ErrBadState = domain.ErrBadState
)
