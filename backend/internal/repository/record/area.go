// Package record contains database row representations private to area storage.
package record

import (
	"database/sql"
	"time"
)

// Area is the sqlx row shape for the areas table.
type Area struct {
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
