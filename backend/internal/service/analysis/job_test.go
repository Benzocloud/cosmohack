package analysis

import (
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func TestNewJobBuildsQueuedSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 20, 30, 123, time.FixedZone("MSK", 3*60*60))
	job, err := NewJob(domain.Area{ID: "area-1", Generation: 4}, domain.Period{From: "2025-01-01", To: "2025-01-02"}, now)
	if err != nil {
		t.Fatalf("NewJob() error = %v", err)
	}
	if job.ID == "" || job.AreaID != "area-1" || job.Status != domain.JobQueued || job.AreaGeneration != 4 {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.CreatedAt.Location() != time.UTC || !job.CreatedAt.Equal(now) || !job.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected timestamps: %+v", job)
	}
}
