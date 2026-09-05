// Пакет record содержит представления строк базы данных, закрытые для хранения участков.
package record

import (
	"database/sql"
	"time"
)

// Area — форма строки sqlx для таблицы areas.
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
