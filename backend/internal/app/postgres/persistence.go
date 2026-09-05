package postgres

import (
	"context"
	"errors"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository"
	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
)

// Persistence adapts repository errors to the executor's consumer-owned port.
type Persistence struct{ repo *repository.Repository }

func NewPersistence(repo *repository.Repository) (*Persistence, error) {
	if repo == nil {
		return nil, errors.New("postgres repository is nil")
	}
	return &Persistence{repo: repo}, nil
}

func (p *Persistence) GetJob(ctx context.Context, id string) (domain.Job, error) {
	job, err := p.repo.GetJob(ctx, id)
	return job, mapExecutorError(err)
}

func (p *Persistence) GetArea(ctx context.Context, id string) (domain.Area, error) {
	area, err := p.repo.GetArea(ctx, id)
	return area, mapExecutorError(err)
}

func (p *Persistence) SetJobRunning(ctx context.Context, id, stage string) error {
	return mapExecutorError(p.repo.SetJobRunning(ctx, id, stage))
}

func (p *Persistence) SetJobStage(ctx context.Context, id, stage string) error {
	return mapExecutorError(p.repo.SetJobStage(ctx, id, stage))
}

func (p *Persistence) SetJobFailed(ctx context.Context, id, code, message string, retryable bool) error {
	return mapExecutorError(p.repo.SetJobFailed(ctx, id, code, message, retryable))
}

func (p *Persistence) SetJobCancelled(ctx context.Context, id string) error {
	return mapExecutorError(p.repo.SetJobCancelled(ctx, id))
}

func (p *Persistence) SetJobInputRevision(ctx context.Context, id, revision string) error {
	return mapExecutorError(p.repo.SetJobInputRevision(ctx, id, revision))
}

func (p *Persistence) PutResult(ctx context.Context, generation int, jobID string, result domain.AnalysisRecord) error {
	return mapExecutorError(p.repo.PutResult(ctx, generation, jobID, result))
}

func (p *Persistence) RecoverInterrupted(ctx context.Context) error {
	return mapExecutorError(p.repo.RecoverInterrupted(ctx))
}

func mapExecutorError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return analysisusecase.ErrNotFound
	case errors.Is(err, repository.ErrBadState):
		return analysisusecase.ErrBadState
	case errors.Is(err, repository.ErrGeneration):
		return analysisusecase.ErrGeneration
	default:
		return err
	}
}

var _ analysisusecase.Persistence = (*Persistence)(nil)
