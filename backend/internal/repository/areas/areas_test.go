package areas

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/repository/areas/record"
)

func TestMapAreaRecord(t *testing.T) {
	shownVersion := sql.NullString{String: "result-1", Valid: true}
	shownJob := sql.NullString{String: "job-1", Valid: true}
	activeJob := sql.NullString{String: "job-2", Valid: true}
	row := record.Area{
		ID:                 "area-1",
		Name:               "field",
		Geometry:           []byte(`{"type":"Polygon","coordinates":[[[30,50],[31,50],[30,50]]]}`),
		Source:             []byte(`{"kind":"drawn"}`),
		PeriodFrom:         time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:           time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		CreatedAt:          time.Date(2026, 5, 1, 10, 11, 12, 0, time.FixedZone("MSK", 3*60*60)),
		Generation:         2,
		ShownResultVersion: shownVersion,
		ShownJobID:         shownJob,
		ActiveJobID:        activeJob,
	}

	area, err := mapAreaRow(row)
	if err != nil {
		t.Fatalf("map area row: %v", err)
	}
	if area.ID != row.ID || area.Period.From != "2026-05-01" || area.Period.To != "2026-05-03" {
		t.Fatalf("unexpected area identity: %+v", area)
	}
	if area.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at must be normalized to UTC: %v", area.CreatedAt.Location())
	}
	if area.ShownResultVersion != "result-1" || area.ShownJobID != "job-1" || area.ActiveJobID != "job-2" {
		t.Fatalf("pointer fields lost: %+v", area)
	}
}

func TestMapAreaRecordRejectsInvalidJSON(t *testing.T) {
	row := record.Area{Geometry: []byte("not-json"), Source: []byte(`{}`)}
	if _, err := mapAreaRow(row); err == nil {
		t.Fatal("invalid geometry JSON must fail mapping")
	}
}
