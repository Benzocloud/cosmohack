package areas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type areaRow struct {
	ID                 string         `db:"id"`
	Name               string         `db:"name"`
	Geometry           []byte         `db:"geometry"`
	Source             []byte         `db:"source"`
	PeriodFrom         time.Time      `db:"period_from"`
	PeriodTo           time.Time      `db:"period_to"`
	CreatedAt          time.Time      `db:"created_at"`
	Generation         int            `db:"generation"`
	ShownResultVersion sql.NullString `db:"shown_result_version"`
	ShownJobID         sql.NullString `db:"shown_job_id"`
	ActiveJobID        sql.NullString `db:"active_job_id"`
}

func (r areaRow) domain() (domain.Area, error) {
	var geometry domain.Polygon
	if err := json.Unmarshal(r.Geometry, &geometry); err != nil {
		return domain.Area{}, fmt.Errorf("decode area geometry: %w", err)
	}
	var source domain.AreaSource
	if err := json.Unmarshal(r.Source, &source); err != nil {
		return domain.Area{}, fmt.Errorf("decode area source: %w", err)
	}
	return domain.Area{
		ID:                 r.ID,
		Name:               r.Name,
		Geometry:           geometry,
		Source:             source,
		Period:             domain.Period{From: formatDate(r.PeriodFrom), To: formatDate(r.PeriodTo)},
		CreatedAt:          r.CreatedAt.UTC(),
		Generation:         r.Generation,
		ShownResultVersion: nullableString(r.ShownResultVersion),
		ShownJobID:         nullableString(r.ShownJobID),
		ActiveJobID:        nullableString(r.ActiveJobID),
	}, nil
}

// Repository stores domain aggregates in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

// New constructs a repository over an already configured sqlx pool.
func New(db *sqlx.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("postgres repository database is nil")
	}
	return &Repository{db: db}, nil
}

// CreateArea persists a new area aggregate.
func (r *Repository) CreateArea(ctx context.Context, area domain.Area) error {
	if err := r.check(); err != nil {
		return err
	}
	geometry, err := json.Marshal(area.Geometry)
	if err != nil {
		return fmt.Errorf("encode area geometry: %w", err)
	}
	source, err := json.Marshal(area.Source)
	if err != nil {
		return fmt.Errorf("encode area source: %w", err)
	}
	from, to, err := parsePeriod(area.Period)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertArea,
		area.ID, area.Name, geometry, source, from, to, area.CreatedAt.UTC(), area.Generation,
		nullableArg(area.ShownResultVersion), nullableArg(area.ShownJobID), nullableArg(area.ActiveJobID),
	)
	if err != nil {
		return fmt.Errorf("insert area: %w", err)
	}
	return nil
}

// GetArea loads an area aggregate by ID.
func (r *Repository) GetArea(ctx context.Context, id string) (domain.Area, error) {
	if err := r.check(); err != nil {
		return domain.Area{}, err
	}
	var row areaRow
	if err := r.db.GetContext(ctx, &row, queryGetArea, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Area{}, ErrNotFound
		}
		return domain.Area{}, fmt.Errorf("get area: %w", err)
	}
	return row.domain()
}

// ListAreas returns areas in the public creation order.
func (r *Repository) ListAreas(ctx context.Context) ([]domain.Area, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	var rows []areaRow
	if err := r.db.SelectContext(ctx, &rows, queryListAreas); err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}
	out := make([]domain.Area, 0, len(rows))
	for _, row := range rows {
		area, err := row.domain()
		if err != nil {
			return nil, err
		}
		out = append(out, area)
	}
	return out, nil
}

func (r *Repository) check() error {
	if r == nil || r.db == nil {
		return errors.New("postgres repository is not configured")
	}
	return nil
}

func parsePeriod(period domain.Period) (time.Time, time.Time, error) {
	from, err := time.Parse("2006-01-02", period.From)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse period from: %w", err)
	}
	to, err := time.Parse("2006-01-02", period.To)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse period to: %w", err)
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("period start is after period end")
	}
	return from, to, nil
}

func formatDate(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}
