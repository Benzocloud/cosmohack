package area

import (
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func TestCreateBuildsInitialArea(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 20, 30, 123, time.FixedZone("MSK", 3*60*60))
	a, err := Create(CreateInput{Name: "field", Period: domain.Period{From: "2025-01-01", To: "2025-01-02"}}, now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if a.ID == "" || a.Generation != 1 || a.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected area identity: %+v", a)
	}
	if !a.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want %v", a.CreatedAt, now)
	}
}
