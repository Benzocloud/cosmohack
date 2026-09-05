package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/record"
	"github.com/jmoiron/sqlx"
)

// PutJobQueued сохраняет новую задачу в очереди и занимает активный слот участка.
// Блокировка участка, вставка и обновление активного указателя выполняются одной транзакцией.
func (r *Repository) PutJobQueued(ctx context.Context, job domain.Job) error {
	return r.putJobQueued(ctx, job, nil)
}

// PutJobQueuedWithPeriod атомарно обновляет период участка по умолчанию и
// занимает его активный слот задачи.
func (r *Repository) PutJobQueuedWithPeriod(ctx context.Context, job domain.Job, period domain.Period) error {
	return r.putJobQueued(ctx, job, &period)
}

func (r *Repository) putJobQueued(ctx context.Context, job domain.Job, areaPeriod *domain.Period) error {
	if err := r.check(); err != nil {
		return err
	}
	if job.ID == "" || job.AreaID == "" {
		return errors.New("job id and area id are required")
	}
	from, to, err := parsePeriod(job.Period)
	if err != nil {
		return err
	}
	created := job.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin queue job: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var area record.Area
	if err := tx.GetContext(ctx, &area, queryLockArea, job.AreaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock area for job: %w", err)
	}
	if area.ActiveJobID.Valid {
		return ErrConflict
	}
	if areaPeriod != nil {
		from, to, err := parsePeriod(*areaPeriod)
		if err != nil {
			return err
		}
		updated, err := tx.ExecContext(ctx, queryUpdateAreaPeriod, job.AreaID, from, to)
		if err != nil {
			return fmt.Errorf("update area period for job: %w", err)
		}
		if err := affected(updated); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, queryInsertJob,
		job.ID, job.AreaID, from, to, area.Generation, created,
	); err != nil {
		return fmt.Errorf("insert job: %w", mapDatabaseError(err))
	}
	if _, err := tx.ExecContext(ctx, querySetActiveJob, job.AreaID, job.ID); err != nil {
		return fmt.Errorf("claim area job slot: %w", mapDatabaseError(err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit queued job: %w", err)
	}
	return nil
}

// GetJob загружает один агрегат задачи.
func (r *Repository) GetJob(ctx context.Context, id string) (domain.Job, error) {
	if err := r.check(); err != nil {
		return domain.Job{}, err
	}
	var row record.Job
	if err := r.db.GetContext(ctx, &row, queryGetJob, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Job{}, ErrNotFound
		}
		return domain.Job{}, fmt.Errorf("get job: %w", err)
	}
	return mapJobRow(row)
}

// DeleteJob удаляет задачу из очереди и освобождает слот участка, если задача всё ещё им владеет.
// Используется для компенсации постановки в очередь после успешного занятия слота.
func (r *Repository) DeleteJob(ctx context.Context, id string) error {
	if err := r.check(); err != nil {
		return err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete job: %w", err)
	}

	defer func() { _ = tx.Rollback() }()
	var key struct {
		AreaID string `db:"area_id"`
	}
	if err := tx.GetContext(ctx, &key, queryJobArea, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("find job area for delete: %w", err)
	}
	var area record.Area
	if err := tx.GetContext(ctx, &area, queryLockArea, key.AreaID); err != nil {
		return fmt.Errorf("lock area for delete job: %w", err)
	}
	var row record.Job
	if err := tx.GetContext(ctx, &row, queryLockJob, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lock job for delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, queryDeleteJob, id); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, queryClearActiveJob, key.AreaID, id); err != nil {
		return fmt.Errorf("release area job slot after delete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete job: %w", err)
	}
	return nil
}

// ListJobsByArea возвращает все задачи участка в порядке создания.
func (r *Repository) ListJobsByArea(ctx context.Context, areaID string) ([]domain.Job, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	var rows []record.Job
	if err := r.db.SelectContext(ctx, &rows, queryListJobsByArea, areaID); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	jobs := make([]domain.Job, 0, len(rows))
	for _, row := range rows {
		job, err := mapJobRow(row)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// SetJobRunning переводит задачу из queued в running.
func (r *Repository) SetJobRunning(ctx context.Context, id, stage string) error {
	return r.transitionJob(ctx, id, func(status domain.JobStatus) bool { return status == domain.JobQueued }, false, func(tx *sqlx.Tx, now time.Time) error {
		_, err := tx.ExecContext(ctx, querySetJobRunning, id, stage, now)
		return err
	})
}

// SetJobStage записывает прогресс выполняемой задачи.
func (r *Repository) SetJobStage(ctx context.Context, id, stage string) error {
	return r.transitionJob(ctx, id, func(status domain.JobStatus) bool { return status == domain.JobRunning }, false, func(tx *sqlx.Tx, now time.Time) error {
		_, err := tx.ExecContext(ctx, querySetJobStage, id, stage, now)
		return err
	})
}

// SetJobFailed переводит активную задачу в failed и освобождает слот участка.
func (r *Repository) SetJobFailed(ctx context.Context, id, code, message string, retryable bool) error {
	return r.transitionJob(ctx, id, func(status domain.JobStatus) bool {
		return status == domain.JobQueued || status == domain.JobRunning
	}, true, func(tx *sqlx.Tx, now time.Time) error {
		_, err := tx.ExecContext(ctx, querySetJobFailed, id, code, message, retryable, now)
		return err
	})
}

// SetJobCancelled переводит активную задачу в cancelled и освобождает слот участка.
func (r *Repository) SetJobCancelled(ctx context.Context, id string) error {
	return r.transitionJob(ctx, id, func(status domain.JobStatus) bool {
		return status == domain.JobQueued || status == domain.JobRunning
	}, true, func(tx *sqlx.Tx, now time.Time) error {
		_, err := tx.ExecContext(ctx, querySetJobCancelled, id, now)
		return err
	})
}

// SetJobInputRevision сохраняет неизменяемую ревизию входа анализа.
func (r *Repository) SetJobInputRevision(ctx context.Context, id, revision string) error {
	if err := r.check(); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, querySetJobInputRevision, id, revision, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set job input revision: %w", err)
	}
	return affected(result)
}

// RecoverInterrupted завершает незаконченные задачи после перезапуска процесса и
// очищает указатели участков, которые больше не ссылаются на активную задачу.
func (r *Repository) RecoverInterrupted(ctx context.Context) error {
	if err := r.check(); err != nil {
		return err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recovery: %w", err)
	}

	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, queryRecoverJobs, time.Now().UTC()); err != nil {
		return fmt.Errorf("recover jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, queryRecoverAreas); err != nil {
		return fmt.Errorf("recover area pointers: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recovery: %w", err)
	}
	return nil
}

type jobUpdate func(*sqlx.Tx, time.Time) error

func (r *Repository) transitionJob(ctx context.Context, id string, allowed func(domain.JobStatus) bool, release bool, update jobUpdate) error {
	if err := r.check(); err != nil {
		return err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin job transition: %w", err)
	}

	defer func() { _ = tx.Rollback() }()
	if release {
		var key struct {
			AreaID string `db:"area_id"`
		}
		if err := tx.GetContext(ctx, &key, queryJobArea, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("find job area: %w", err)
		}
		var area record.Area
		if err := tx.GetContext(ctx, &area, queryLockArea, key.AreaID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("lock area for job transition: %w", err)
		}
	}

	var row record.Job
	if err := tx.GetContext(ctx, &row, queryLockJob, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock job: %w", err)
	}
	if !allowed(domain.JobStatus(row.Status)) {
		return ErrBadState
	}
	if err := update(tx, time.Now().UTC()); err != nil {
		return fmt.Errorf("update job: %w", mapDatabaseError(err))
	}
	if release {
		if _, err := tx.ExecContext(ctx, queryClearActiveJob, row.AreaID, id); err != nil {
			return fmt.Errorf("release area job slot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit job transition: %w", err)
	}
	return nil
}

func affected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}
