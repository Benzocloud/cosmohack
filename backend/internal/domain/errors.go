package domain

import "errors"

// Ошибки хранения, общие для границ приложения.
var (
	ErrNotFound   = errors.New("record not found")
	ErrConflict   = errors.New("record conflict")
	ErrBadState   = errors.New("invalid record state")
	ErrGeneration = errors.New("area generation changed")
)
