package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/record"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

// Repository stores domain aggregates in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

// New constructs a repository over an already configured sqlx pool.
func New(db *sqlx.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("repository database is nil")
	}
	return &Repository{db: db}, nil
}

// CreateArea persists a new area aggregate.
func (r *Repository) CreateArea(ctx context.Context, area domain.Area) error {
	if err := r.check(); err != nil {
		return err
	}
	row, err := newAreaRow(area)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertArea,
		row.ID, row.Name, row.Geometry, row.Source, row.PeriodFrom, row.PeriodTo, row.CreatedAt,
		row.Generation, nullableArg(row.ShownResultVersion), nullableArg(row.ShownJobID), nullableArg(row.ActiveJobID),
	)
	if err != nil {
		return fmt.Errorf("insert area: %w", mapDatabaseError(err))
	}
	return nil
}

func mapDatabaseError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

// GetArea loads an area aggregate by ID.
func (r *Repository) GetArea(ctx context.Context, id string) (domain.Area, error) {
	if err := r.check(); err != nil {
		return domain.Area{}, err
	}
	var row record.Area
	if err := r.db.GetContext(ctx, &row, queryGetArea, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Area{}, ErrNotFound
		}
		return domain.Area{}, fmt.Errorf("get area: %w", err)
	}
	return mapAreaRow(row)
}

// ListAreas returns areas in the public creation order.
func (r *Repository) ListAreas(ctx context.Context) ([]domain.Area, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	var rows []record.Area
	if err := r.db.SelectContext(ctx, &rows, queryListAreas); err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}
	out := make([]domain.Area, 0, len(rows))
	for _, row := range rows {
		area, err := mapAreaRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, area)
	}
	return out, nil
}

// DeleteArea cancels active jobs, removes immutable results and deletes the
// area in one transaction. Job history intentionally remains queryable.
func (r *Repository) DeleteArea(ctx context.Context, id string) ([]string, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin delete area: %w", err)
	}
	defer tx.Rollback()
	var area record.Area
	if err := tx.GetContext(ctx, &area, queryLockArea, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lock area for delete: %w", err)
	}
	var active []struct {
		ID string `db:"id"`
	}
	if err := tx.SelectContext(ctx, &active, queryActiveJobsByArea, id); err != nil {
		return nil, fmt.Errorf("list active area jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, queryCancelAreaJobs, id, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("cancel area jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, queryDeleteAreaResults, id); err != nil {
		return nil, fmt.Errorf("delete area results: %w", err)
	}
	deleted, err := tx.ExecContext(ctx, queryDeleteArea, id)
	if err != nil {
		return nil, fmt.Errorf("delete area: %w", err)
	}
	if err := affected(deleted); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit delete area: %w", err)
	}
	ids := make([]string, 0, len(active))
	for _, job := range active {
		ids = append(ids, job.ID)
	}
	return ids, nil
}

func (r *Repository) check() error {
	if r == nil || r.db == nil {
		return errors.New("repository is not configured")
	}
	return nil
}
