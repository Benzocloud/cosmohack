package handler

import (
	"errors"

	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
)

var (
	errStorageNotFound = analysisusecase.ErrNotFound
	errStorageConflict = errors.New("storage conflict")
	errStorageBadState = errors.New("storage bad state")
)

// Exported aliases let the composition root normalize repository errors
// without making handlers depend on a concrete persistence package.
var (
	ErrStorageNotFound = errStorageNotFound
	ErrStorageConflict = errStorageConflict
	ErrStorageBadState = errStorageBadState
)
