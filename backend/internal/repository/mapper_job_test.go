package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/record"
)

func TestMapJobRecord(t *testing.T) {
	retryable := true
	row := record.Job{
		ID:             "job-1",
		AreaID:         "area-1",
		Status:         string(domain.JobFailed),
		Stage:          sql.NullString{String: domain.StageAnalyze, Valid: true},
		PeriodFrom:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:       time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		ErrorCode:      sql.NullString{String: "ml_timeout", Valid: true},
		ErrorMessage:   sql.NullString{String: "timeout", Valid: true},
		ErrorRetryable: sql.NullBool{Bool: retryable, Valid: true},
		InputRevision:  sql.NullString{String: "rev-1", Valid: true},
	}

	job, err := mapJobRow(row)
	if err != nil {
		t.Fatalf("map job row: %v", err)
	}
	if job.ID != row.ID || job.Status != domain.JobFailed || job.Period.From != "2026-05-01" {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.Stage == nil || *job.Stage != domain.StageAnalyze || job.ErrorRetryable == nil || !*job.ErrorRetryable {
		t.Fatalf("nullable job fields lost: %+v", job)
	}
}

func TestResultRowRoundTrip(t *testing.T) {
	severity := domain.SeverityModerate
	want := domain.AnalysisRecord{
		AreaID: "area-1", ResultVersion: "result-1", Period: domain.Period{From: "2026-05-01", To: "2026-05-03"},
		ComputedAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), SchemaVersion: domain.SchemaVersionV1,
		FeatureProfile: domain.FeatureProfileNDVIWeatherV1, ModelVersion: "model-1", Method: "baseline",
		Status: domain.StatusCandidate, Severity: &severity,
		Series:  []domain.SeriesPoint{{Date: "2026-05-01", State: domain.StateObserved}},
		Weather: []domain.WeatherPoint{{Date: "2026-05-01"}}, Provenance: map[string]any{"source": "fixture"},
		Limitations: []string{"clouds"}, Events: []domain.AnomalyEvent{{StartDate: "2026-05-02", EndDate: "2026-05-03", Status: domain.StatusCandidate, Severity: domain.SeverityModerate}},
		InputRevision: "rev-1",
	}
	row, err := newResultRow(want)
	if err != nil {
		t.Fatalf("new result row: %v", err)
	}
	got, err := mapResultRow(row)
	if err != nil {
		t.Fatalf("map result row: %v", err)
	}
	if got.AreaID != want.AreaID || got.ResultVersion != want.ResultVersion || got.Status != want.Status || got.Severity == nil || *got.Severity != severity {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestMapJobRecordRejectsUnknownStatus(t *testing.T) {
	row := record.Job{
		Status:     "unknown",
		PeriodFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := mapJobRow(row); err == nil {
		t.Fatal("unknown job status must fail mapping")
	}
}
