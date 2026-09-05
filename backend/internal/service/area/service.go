// Package area contains area use-case construction and invariants.
package area

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// Repository is the consumer-owned persistence port for area use cases.
type Repository interface {
	CreateArea(context.Context, domain.Area) error
	GetArea(context.Context, string) (domain.Area, error)
	ListAreas(context.Context) ([]domain.Area, error)
	DeleteArea(context.Context, string) ([]string, error)
}

// Service owns area use cases and delegates persistence to Repository.
type Service struct {
	repo Repository
	now  func() time.Time
}

// New constructs the area use-case service.
func New(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// CreateArea builds and persists a new area.
func (s *Service) CreateArea(ctx context.Context, input CreateInput) (domain.Area, error) {
	area, err := Create(input, s.now())
	if err != nil {
		return domain.Area{}, err
	}
	if err := s.repo.CreateArea(ctx, area); err != nil {
		return domain.Area{}, err
	}
	return area, nil
}

// GetArea loads one area.
func (s *Service) GetArea(ctx context.Context, id string) (domain.Area, error) {
	return s.repo.GetArea(ctx, id)
}

// ListAreas loads all areas.
func (s *Service) ListAreas(ctx context.Context) ([]domain.Area, error) {
	return s.repo.ListAreas(ctx)
}

// DeleteArea removes an area and returns jobs that were cancelled by storage.
func (s *Service) DeleteArea(ctx context.Context, id string) ([]string, error) {
	return s.repo.DeleteArea(ctx, id)
}

// CreateInput contains validated user input for a new area.
type CreateInput struct {
	Name     string
	Period   domain.Period
	Geometry domain.Polygon
	Source   domain.AreaSource
}

// Create builds a new area aggregate with its initial generation.
func Create(input CreateInput, now time.Time) (domain.Area, error) {
	id, err := newID()
	if err != nil {
		return domain.Area{}, err
	}
	return domain.Area{
		ID:         id,
		Name:       input.Name,
		Geometry:   input.Geometry,
		Source:     input.Source,
		Period:     input.Period,
		CreatedAt:  now.UTC(),
		Generation: 1,
	}, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
