package record

import (
	"database/sql"
	"time"
)

// AnalysisResult is the sqlx row shape for an immutable analysis result.
type AnalysisResult struct {
	AreaID         string         `db:"area_id"`
	ResultVersion  string         `db:"result_version"`
	PeriodFrom     time.Time      `db:"period_from"`
	PeriodTo       time.Time      `db:"period_to"`
	ComputedAt     time.Time      `db:"computed_at"`
	SchemaVersion  string         `db:"schema_version"`
	FeatureProfile string         `db:"feature_profile"`
	ModelVersion   string         `db:"model_version"`
	Method         string         `db:"method"`
	Status         string         `db:"status"`
	Severity       sql.NullString `db:"severity"`
	InputRevision  string         `db:"input_revision"`
	ContentHash    string         `db:"content_hash"`
	Series         []byte         `db:"series"`
	Weather        []byte         `db:"weather"`
	Provenance     []byte         `db:"provenance"`
	Limitations    []byte         `db:"limitations"`
	Events         []byte         `db:"events"`
}
