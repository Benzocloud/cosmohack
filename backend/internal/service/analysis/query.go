package analysis

import (
	"context"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// QueryPersistence — порт чтения, используемый проекциями анализа.
type QueryPersistence interface {
	GetJob(context.Context, string) (domain.Job, error)
	GetResult(context.Context, string, string) (domain.AnalysisRecord, error)
}

// QueryService владеет сценариями чтения анализа, которые используют HTTP-обработчики.
type QueryService struct {
	persistence QueryPersistence
}

// NewQueryService создаёт сервис запросов анализа.
func NewQueryService(persistence QueryPersistence) *QueryService {
	return &QueryService{persistence: persistence}
}

// GetJob загружает задачу для публичной проекции.
func (s *QueryService) GetJob(ctx context.Context, id string) (domain.Job, error) {
	return s.persistence.GetJob(ctx, id)
}

// GetResult загружает неизменяемый результат анализа для публичной проекции.
func (s *QueryService) GetResult(ctx context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	return s.persistence.GetResult(ctx, areaID, version)
}
