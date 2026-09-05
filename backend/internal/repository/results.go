package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/record"
)

// GetResult loads an immutable analysis record.
func (r *Repository) GetResult(ctx context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	if err := r.check(); err != nil {
		return domain.AnalysisRecord{}, err
	}
	var row record.AnalysisResult
	if err := r.db.GetContext(ctx, &row, queryGetResult, areaID, version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AnalysisRecord{}, ErrNotFound
		}
		return domain.AnalysisRecord{}, fmt.Errorf("get analysis result: %w", err)
	}
	return mapResultRow(row)
}

// PutResult atomically persists an immutable result, completes its running job,
// and publishes the version on the area. Replaying an identical version is
// idempotent; a different payload under the same key returns ErrConflict.
func (r *Repository) PutResult(ctx context.Context, generationAtStart int, jobID string, result domain.AnalysisRecord) error {
	if err := r.check(); err != nil {
		return err
	}
	if result.AreaID == "" || result.ResultVersion == "" || jobID == "" || result.InputRevision == "" ||
		result.SchemaVersion == "" || result.FeatureProfile == "" || result.ModelVersion == "" || result.Method == "" {
		return errors.New("result required fields are missing")
	}
	row, err := newResultRow(result)
	if err != nil {
		return err
	}
	if row.ComputedAt.IsZero() {
		row.ComputedAt = time.Now().UTC()
	}
	row.ContentHash = hashResult(row)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save result: %w", err)
	}
	defer tx.Rollback()

	var area record.Area
	if err := tx.GetContext(ctx, &area, queryLockArea, result.AreaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock area for result: %w", err)
	}
	if area.Generation != generationAtStart {
		return ErrGeneration
	}
	var job record.Job
	if err := tx.GetContext(ctx, &job, queryLockJob, jobID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock result job: %w", err)
	}
	if job.AreaID != result.AreaID || job.Status != string(domain.JobRunning) || job.AreaGeneration != generationAtStart {
		return ErrBadState
	}

	if _, err := tx.ExecContext(ctx, queryInsertResult,
		row.AreaID, row.ResultVersion, row.PeriodFrom, row.PeriodTo, row.ComputedAt,
		row.SchemaVersion, row.FeatureProfile, row.ModelVersion, row.Method, row.Status,
		nullableArg(row.Severity), row.InputRevision, row.ContentHash,
		row.Series, row.Weather, row.Provenance, row.Limitations, row.Events,
	); err != nil {
		return fmt.Errorf("insert analysis result: %w", mapDatabaseError(err))
	}

	var existing record.AnalysisResult
	if err := tx.GetContext(ctx, &existing, queryLockResult, row.AreaID, row.ResultVersion); err != nil {
		return fmt.Errorf("read saved analysis result: %w", err)
	}
	if existing.ContentHash != row.ContentHash {
		return ErrConflict
	}
	completed, err := tx.ExecContext(ctx, querySetJobCompleted, jobID, row.ResultVersion, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("complete analysis job: %w", mapDatabaseError(err))
	}
	if err := affected(completed); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrBadState
		}
		return fmt.Errorf("check completed job: %w", err)
	}
	published, err := tx.ExecContext(ctx, queryPublishResult, result.AreaID, row.ResultVersion, jobID)
	if err != nil {
		return fmt.Errorf("publish analysis result: %w", err)
	}
	if err := affected(published); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrBadState
		}
		return fmt.Errorf("check published result: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit analysis result: %w", err)
	}
	return nil
}

// ErrGeneration indicates that an area changed while analysis was running.
var ErrGeneration = errors.New("area generation changed")

type resultHashInput struct {
	AreaID, ResultVersion, PeriodFrom, PeriodTo                 string
	SchemaVersion, FeatureProfile, ModelVersion, Method, Status string
	Severity, InputRevision                                     string
	Series, Weather, Provenance, Limitations, Events            json.RawMessage
}

func hashResult(row record.AnalysisResult) string {
	input, _ := json.Marshal(resultHashInput{
		AreaID: row.AreaID, ResultVersion: row.ResultVersion,
		PeriodFrom: row.PeriodFrom.UTC().Format("2006-01-02"), PeriodTo: row.PeriodTo.UTC().Format("2006-01-02"),
		SchemaVersion:  row.SchemaVersion,
		FeatureProfile: row.FeatureProfile, ModelVersion: row.ModelVersion, Method: row.Method, Status: row.Status,
		Severity: row.Severity.String, InputRevision: row.InputRevision,
		Series: row.Series, Weather: row.Weather, Provenance: row.Provenance,
		Limitations: row.Limitations, Events: row.Events,
	})
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}
