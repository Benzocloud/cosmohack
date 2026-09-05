package postgres

import (
	"context"
	"errors"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/repository"
)

// Storage adapts the PostgreSQL repository to the handler's domain port.
// Error normalization stays at the composition boundary.
type Storage struct{ repo *repository.Repository }

func NewStorage(repo *repository.Repository) (*Storage, error) {
	if repo == nil {
		return nil, errors.New("postgres repository is nil")
	}
	return &Storage{repo: repo}, nil
}

func (s *Storage) CreateArea(ctx context.Context, area domain.Area) error {
	return mapHandlerError(s.repo.CreateArea(ctx, area))
}

func (s *Storage) UpdateArea(ctx context.Context, area domain.Area) error {
	return mapHandlerError(s.repo.UpdateArea(ctx, area))
}

func (s *Storage) GetArea(ctx context.Context, id string) (domain.Area, error) {
	area, err := s.repo.GetArea(ctx, id)
	return area, mapHandlerError(err)
}

func (s *Storage) ListAreas(ctx context.Context) ([]domain.Area, error) {
	areas, err := s.repo.ListAreas(ctx)
	return areas, mapHandlerError(err)
}

func (s *Storage) DeleteArea(ctx context.Context, id string) ([]string, error) {
	ids, err := s.repo.DeleteArea(ctx, id)
	return ids, mapHandlerError(err)
}

func (s *Storage) GetJob(ctx context.Context, id string) (domain.Job, error) {
	job, err := s.repo.GetJob(ctx, id)
	return job, mapHandlerError(err)
}

func (s *Storage) PutJobQueued(ctx context.Context, job domain.Job) error {
	return mapHandlerError(s.repo.PutJobQueued(ctx, job))
}

func (s *Storage) DeleteJob(ctx context.Context, id string) error {
	return mapHandlerError(s.repo.DeleteJob(ctx, id))
}

func (s *Storage) GetResult(ctx context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	result, err := s.repo.GetResult(ctx, areaID, version)
	return result, mapHandlerError(err)
}

func mapHandlerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return handler.ErrStorageNotFound
	case errors.Is(err, repository.ErrConflict):
		return handler.ErrStorageConflict
	case errors.Is(err, repository.ErrBadState):
		return handler.ErrStorageBadState
	default:
		return err
	}
}

var _ handler.Storage = (*Storage)(nil)
