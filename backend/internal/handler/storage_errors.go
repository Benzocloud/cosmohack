package handler

import (
	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

var (
	errStorageNotFound = domain.ErrNotFound
	errStorageConflict = domain.ErrConflict
	errStorageBadState = domain.ErrBadState
)

// Exported aliases let the composition root normalize repository errors
// without making handlers depend on a concrete persistence package.
var (
	ErrStorageNotFound = errStorageNotFound
	ErrStorageConflict = errStorageConflict
	ErrStorageBadState = errStorageBadState
)
