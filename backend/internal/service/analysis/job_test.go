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

func TestResolvePeriod(t *testing.T) {
	area := domain.Area{Period: domain.Period{From: "2024-01-01", To: "2024-01-31"}}
	if got := ResolvePeriod(area, nil); got != area.Period {
		t.Fatalf("default period = %+v", got)
	}
	want := domain.Period{From: "2025-02-01", To: "2025-02-28"}
	if got := ResolvePeriod(area, &want); got != want {
		t.Fatalf("requested period = %+v", got)
	}
}

func TestIsActiveStatus(t *testing.T) {
	if !IsActiveStatus(domain.JobQueued) || !IsActiveStatus(domain.JobRunning) {
		t.Fatal("queued and running jobs must be active")
	}
	if IsActiveStatus(domain.JobCompleted) || IsActiveStatus(domain.JobFailed) || IsActiveStatus(domain.JobCancelled) {
		t.Fatal("terminal jobs must not be active")
	}
}
