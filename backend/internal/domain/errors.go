package domain

import "errors"

// Persistence errors shared across application boundaries.
var (
	ErrNotFound   = errors.New("record not found")
	ErrConflict   = errors.New("record conflict")
	ErrBadState   = errors.New("invalid record state")
	ErrGeneration = errors.New("area generation changed")
)
