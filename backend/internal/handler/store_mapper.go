package handler

import (
	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// storeJobFromDomain is a temporary adapter used until PostgreSQL repository
// becomes the persistence boundary.
func storeJobFromDomain(job domain.Job) store.Job {
	return store.Job{
		ID:             job.ID,
		AreaID:         job.AreaID,
		Status:         store.JobQueued,
		Period:         store.Period(job.Period),
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		AreaGeneration: job.AreaGeneration,
	}
}
