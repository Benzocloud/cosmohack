package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/repository/record"
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

func mapJobRow(row record.Job) (domain.Job, error) {
	status := domain.JobStatus(row.Status)
	switch status {
	case domain.JobQueued, domain.JobRunning, domain.JobCompleted, domain.JobFailed, domain.JobCancelled:
	default:
		return domain.Job{}, fmt.Errorf("decode job status: %q", row.Status)
	}
	from := formatDate(row.PeriodFrom)
	to := formatDate(row.PeriodTo)
	if row.PeriodFrom.IsZero() || row.PeriodTo.IsZero() {
		return domain.Job{}, errors.New("decode job period: missing date")
	}
	return domain.Job{
		ID:             row.ID,
		AreaID:         row.AreaID,
		Status:         status,
		Period:         domain.Period{From: from, To: to},
		CreatedAt:      row.CreatedAt.UTC(),
		UpdatedAt:      row.UpdatedAt.UTC(),
		AreaGeneration: row.AreaGeneration,
		Stage:          nullableStringPtr(row.Stage),
		ErrorCode:      nullableStringPtr(row.ErrorCode),
		ErrorMessage:   nullableStringPtr(row.ErrorMessage),
		ErrorRetryable: nullableBoolPtr(row.ErrorRetryable),
		ResultVersion:  nullableStringPtr(row.ResultVersion),
		InputRevision:  nullableStringPtr(row.InputRevision),
	}, nil
}

func mapResultRow(row record.AnalysisResult) (domain.AnalysisRecord, error) {
	status := domain.ResultStatus(row.Status)
	switch status {
	case domain.StatusNormal, domain.StatusCandidate, domain.StatusConfirmed, domain.StatusInsufficientData:
	default:
		return domain.AnalysisRecord{}, fmt.Errorf("decode result status: %q", row.Status)
	}
	var series []domain.SeriesPoint
	var weather []domain.WeatherPoint
	var provenance map[string]any
	var limitations []string
	var events []domain.AnomalyEvent
	if row.PeriodFrom.IsZero() || row.PeriodTo.IsZero() {
		return domain.AnalysisRecord{}, errors.New("decode result period: missing date")
	}
	for _, payload := range []struct {
		name string
		data []byte
	}{
		{name: "series", data: row.Series}, {name: "weather", data: row.Weather},
		{name: "provenance", data: row.Provenance}, {name: "limitations", data: row.Limitations},
		{name: "events", data: row.Events},
	} {
		if len(payload.data) == 0 {
			return domain.AnalysisRecord{}, fmt.Errorf("decode result %s: empty payload", payload.name)
		}
	}
	if err := json.Unmarshal(row.Series, &series); err != nil {
		return domain.AnalysisRecord{}, fmt.Errorf("decode result series: %w", err)
	}
	if err := json.Unmarshal(row.Weather, &weather); err != nil {
		return domain.AnalysisRecord{}, fmt.Errorf("decode result weather: %w", err)
	}
	if err := json.Unmarshal(row.Provenance, &provenance); err != nil {
		return domain.AnalysisRecord{}, fmt.Errorf("decode result provenance: %w", err)
	}
	if err := json.Unmarshal(row.Limitations, &limitations); err != nil {
		return domain.AnalysisRecord{}, fmt.Errorf("decode result limitations: %w", err)
	}
	if err := json.Unmarshal(row.Events, &events); err != nil {
		return domain.AnalysisRecord{}, fmt.Errorf("decode result events: %w", err)
	}
	var severity *domain.Severity
	if row.Severity.Valid {
		value := domain.Severity(row.Severity.String)
		severity = &value
	}
	return domain.AnalysisRecord{
		AreaID: row.AreaID, ResultVersion: row.ResultVersion,
		Period:     domain.Period{From: formatDate(row.PeriodFrom), To: formatDate(row.PeriodTo)},
		ComputedAt: row.ComputedAt.UTC(), SchemaVersion: row.SchemaVersion,
		FeatureProfile: row.FeatureProfile, ModelVersion: row.ModelVersion, Method: row.Method,
		Status: status, Severity: severity, InputRevision: row.InputRevision,
		Series: series, Weather: weather, Provenance: provenance, Limitations: limitations, Events: events,
	}, nil
}

func newResultRow(result domain.AnalysisRecord) (record.AnalysisResult, error) {
	from, to, err := parsePeriod(result.Period)
	if err != nil {
		return record.AnalysisResult{}, err
	}
	series, err := json.Marshal(result.Series)
	if err != nil {
		return record.AnalysisResult{}, fmt.Errorf("encode result series: %w", err)
	}
	weather, err := json.Marshal(result.Weather)
	if err != nil {
		return record.AnalysisResult{}, fmt.Errorf("encode result weather: %w", err)
	}
	provenance, err := json.Marshal(result.Provenance)
	if err != nil {
		return record.AnalysisResult{}, fmt.Errorf("encode result provenance: %w", err)
	}
	limitations, err := json.Marshal(result.Limitations)
	if err != nil {
		return record.AnalysisResult{}, fmt.Errorf("encode result limitations: %w", err)
	}
	events, err := json.Marshal(result.Events)
	if err != nil {
		return record.AnalysisResult{}, fmt.Errorf("encode result events: %w", err)
	}
	return record.AnalysisResult{
		AreaID: result.AreaID, ResultVersion: result.ResultVersion, PeriodFrom: from, PeriodTo: to,
		ComputedAt: result.ComputedAt.UTC(), SchemaVersion: result.SchemaVersion,
		FeatureProfile: result.FeatureProfile, ModelVersion: result.ModelVersion, Method: result.Method,
		Status: string(result.Status), Severity: nullableSeverity(result.Severity), InputRevision: result.InputRevision,
		Series: series, Weather: weather, Provenance: provenance, Limitations: limitations, Events: events,
	}, nil
}

func nullableSeverity(value *domain.Severity) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*value), Valid: true}
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

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func nullableBoolPtr(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	result := value.Bool
	return &result
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
