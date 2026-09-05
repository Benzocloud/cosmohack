package areas

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/areas/record"
)

func mapAreaRow(row record.Area) (domain.Area, error) {
	var geometry domain.Polygon
	if err := json.Unmarshal(row.Geometry, &geometry); err != nil {
		return domain.Area{}, fmt.Errorf("decode area geometry: %w", err)
	}
	var source domain.AreaSource
	if err := json.Unmarshal(row.Source, &source); err != nil {
		return domain.Area{}, fmt.Errorf("decode area source: %w", err)
	}
	return domain.Area{
		ID:                 row.ID,
		Name:               row.Name,
		Geometry:           geometry,
		Source:             source,
		Period:             domain.Period{From: formatDate(row.PeriodFrom), To: formatDate(row.PeriodTo)},
		CreatedAt:          row.CreatedAt.UTC(),
		Generation:         row.Generation,
		ShownResultVersion: nullableString(row.ShownResultVersion),
		ShownJobID:         nullableString(row.ShownJobID),
		ActiveJobID:        nullableString(row.ActiveJobID),
	}, nil
}

func newAreaRow(area domain.Area) (record.Area, error) {
	geometry, err := json.Marshal(area.Geometry)
	if err != nil {
		return record.Area{}, fmt.Errorf("encode area geometry: %w", err)
	}
	source, err := json.Marshal(area.Source)
	if err != nil {
		return record.Area{}, fmt.Errorf("encode area source: %w", err)
	}
	from, to, err := parsePeriod(area.Period)
	if err != nil {
		return record.Area{}, err
	}
	return record.Area{
		ID:                 area.ID,
		Name:               area.Name,
		Geometry:           geometry,
		Source:             source,
		PeriodFrom:         from,
		PeriodTo:           to,
		CreatedAt:          area.CreatedAt.UTC(),
		Generation:         area.Generation,
		ShownResultVersion: nullableValue(area.ShownResultVersion),
		ShownJobID:         nullableValue(area.ShownJobID),
		ActiveJobID:        nullableValue(area.ActiveJobID),
	}, nil
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

func nullableValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableArg(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
