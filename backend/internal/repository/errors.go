package repository

import "errors"

// ErrNotFound indicates that a requested aggregate does not exist.
var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record conflict")
	ErrBadState = errors.New("invalid record state")
)
