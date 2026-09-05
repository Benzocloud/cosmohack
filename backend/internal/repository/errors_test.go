package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapDatabaseErrorUniqueViolation(t *testing.T) {
	err := mapDatabaseError(&pgconn.PgError{Code: "23505"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("unique violation = %v, want ErrConflict", err)
	}
}

func TestMapDatabaseErrorPreservesOtherErrors(t *testing.T) {
	original := errors.New("database unavailable")
	if got := mapDatabaseError(original); !errors.Is(got, original) {
		t.Fatalf("error = %v, want original", got)
	}
}
