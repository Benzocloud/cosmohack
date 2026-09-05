package analysis

import (
	"context"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// QueryPersistence is the read port used by analysis projections.
type QueryPersistence interface {
	GetJob(context.Context, string) (domain.Job, error)
	GetResult(context.Context, string, string) (domain.AnalysisRecord, error)
}

// QueryService owns analysis read use cases consumed by HTTP handlers.
type QueryService struct {
	persistence QueryPersistence
}

// NewQueryService constructs the analysis query service.
func NewQueryService(persistence QueryPersistence) *QueryService {
	return &QueryService{persistence: persistence}
}

// GetJob loads a job for its public projection.
func (s *QueryService) GetJob(ctx context.Context, id string) (domain.Job, error) {
	return s.persistence.GetJob(ctx, id)
}

// GetResult loads an immutable analysis result for its public projection.
func (s *QueryService) GetResult(ctx context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	return s.persistence.GetResult(ctx, areaID, version)
}
