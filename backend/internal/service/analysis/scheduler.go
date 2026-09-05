package analysis

import (
	"context"
	"errors"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// SchedulerPersistence is the small write port needed to accept an analysis.
type SchedulerPersistence interface {
	GetArea(context.Context, string) (domain.Area, error)
	GetJob(context.Context, string) (domain.Job, error)
	UpdateArea(context.Context, domain.Area) error
	PutJobQueued(context.Context, domain.Job) error
	DeleteJob(context.Context, string) error
}

type Enqueuer interface {
	Enqueue(context.Context, string) error
}

// Scheduler owns the start-analysis use case and its queue compensation.
type Scheduler struct {
	persistence SchedulerPersistence
	queue       Enqueuer
	now         func() time.Time
}

func NewScheduler(persistence SchedulerPersistence, queue Enqueuer) *Scheduler {
	return &Scheduler{persistence: persistence, queue: queue, now: time.Now}
}

func (s *Scheduler) Start(ctx context.Context, areaID string, requestedPeriod *domain.Period) (domain.Job, error) {
	area, err := s.persistence.GetArea(ctx, areaID)
	if err != nil {
		return domain.Job{}, err
	}
	if area.ActiveJobID != "" {
		job, jobErr := s.persistence.GetJob(ctx, area.ActiveJobID)
		if jobErr == nil && IsActiveStatus(job.Status) {
			return domain.Job{}, ErrConflict
		}
		if jobErr != nil && !errors.Is(jobErr, ErrNotFound) {
			return domain.Job{}, jobErr
		}
		area.ActiveJobID = ""
		if err := s.persistence.UpdateArea(ctx, area); err != nil {
			return domain.Job{}, err
		}
	}

	job, err := NewJob(area, ResolvePeriod(area, requestedPeriod), s.now().UTC())
	if err != nil {
		return domain.Job{}, err
	}
	if err := s.persistence.PutJobQueued(ctx, job); err != nil {
		return domain.Job{}, err
	}
	if err := s.queue.Enqueue(ctx, job.ID); err != nil {
		if deleteErr := s.persistence.DeleteJob(ctx, job.ID); deleteErr != nil {
			return domain.Job{}, deleteErr
		}
		return domain.Job{}, err
	}
	if requestedPeriod != nil {
		fresh, err := s.persistence.GetArea(ctx, areaID)
		if err != nil {
			return domain.Job{}, err
		}
		fresh.Period = *requestedPeriod
		if err := s.persistence.UpdateArea(ctx, fresh); err != nil {
			return domain.Job{}, err
		}
	}
	return job, nil
}
