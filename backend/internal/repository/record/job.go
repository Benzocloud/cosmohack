package record

import (
	"database/sql"
	"time"
)

// Job is the sqlx row shape for the jobs table.
type Job struct {
	ID             string         `db:"id"`
	AreaID         string         `db:"area_id"`
	Status         string         `db:"status"`
	Stage          sql.NullString `db:"stage"`
	PeriodFrom     time.Time      `db:"period_from"`
	PeriodTo       time.Time      `db:"period_to"`
	ErrorCode      sql.NullString `db:"error_code"`
	ErrorMessage   sql.NullString `db:"error_message"`
	ErrorRetryable sql.NullBool   `db:"error_retryable"`
	ResultVersion  sql.NullString `db:"result_version"`
	AreaGeneration int            `db:"area_generation"`
	InputRevision  sql.NullString `db:"input_revision"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}
