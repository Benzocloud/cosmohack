// Пакет area содержит сценарии работы с участками и их инварианты.
package area

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// Repository — порт хранения сценариев участков, которым владеет потребитель.
type Repository interface {
	CreateArea(context.Context, domain.Area) error
	GetArea(context.Context, string) (domain.Area, error)
	ListAreas(context.Context) ([]domain.Area, error)
	DeleteArea(context.Context, string) ([]string, error)
}

// Service владеет сценариями участков и делегирует хранение Repository.
type Service struct {
	repo Repository
	now  func() time.Time
}

// New создаёт сервис сценариев участков.
func New(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// CreateArea создаёт и сохраняет новый участок.
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

// GetArea загружает один участок.
func (s *Service) GetArea(ctx context.Context, id string) (domain.Area, error) {
	return s.repo.GetArea(ctx, id)
}

// ListAreas загружает все участки.
func (s *Service) ListAreas(ctx context.Context) ([]domain.Area, error) {
	return s.repo.ListAreas(ctx)
}

// DeleteArea удаляет участок и возвращает задачи, отменённые хранилищем.
func (s *Service) DeleteArea(ctx context.Context, id string) ([]string, error) {
	return s.repo.DeleteArea(ctx, id)
}

// CreateInput содержит проверенный пользовательский ввод для нового участка.
type CreateInput struct {
	Name     string
	Period   domain.Period
	Geometry domain.Polygon
	Source   domain.AreaSource
}

// Create создаёт новый агрегат участка с начальным поколением.
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
