package handler

import "errors"

var (
	errStorageNotFound = errors.New("storage not found")
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
