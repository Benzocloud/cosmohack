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

func storeAreaFromDomain(area domain.Area) store.Area {
	return store.Area{
		ID: area.ID, Name: area.Name, Geometry: store.Polygon(area.Geometry),
		Source: store.Source{Kind: area.Source.Kind, ContourID: area.Source.ContourID, Provider: area.Source.Provider},
		Period: store.Period(area.Period), CreatedAt: area.CreatedAt,
		Generation: area.Generation, ShownResultVersion: area.ShownResultVersion, ActiveJobID: area.ActiveJobID,
	}
}
