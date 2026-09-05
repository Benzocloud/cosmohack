package repository

import "errors"

// ErrNotFound indicates that a requested aggregate does not exist.
var ErrNotFound = errors.New("postgres record not found")
