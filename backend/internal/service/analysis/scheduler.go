package analysis

import (
	"context"
	"errors"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// SchedulerPersistence — небольшой порт записи, необходимый для принятия анализа.
type SchedulerPersistence interface {
	GetArea(context.Context, string) (domain.Area, error)
	GetJob(context.Context, string) (domain.Job, error)
	UpdateArea(context.Context, domain.Area) error
	PutJobQueued(context.Context, domain.Job) error
	PutJobQueuedWithPeriod(context.Context, domain.Job, domain.Period) error
	DeleteJob(context.Context, string) error
}

type Enqueuer interface {
	Enqueue(context.Context, string) error
}

// Scheduler владеет сценарием запуска анализа и компенсацией очереди.
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
	queueErr := error(nil)
	if requestedPeriod != nil {
		queueErr = s.persistence.PutJobQueuedWithPeriod(ctx, job, *requestedPeriod)
	} else {
		queueErr = s.persistence.PutJobQueued(ctx, job)
	}
	if queueErr != nil {
		return domain.Job{}, queueErr
	}
	if err := s.queue.Enqueue(ctx, job.ID); err != nil {
		if deleteErr := s.persistence.DeleteJob(ctx, job.ID); deleteErr != nil {
			return domain.Job{}, deleteErr
		}
		if requestedPeriod != nil {
			if restoreErr := s.persistence.UpdateArea(ctx, area); restoreErr != nil {
				return domain.Job{}, restoreErr
			}
		}
		return domain.Job{}, err
	}
	return job, nil
}
