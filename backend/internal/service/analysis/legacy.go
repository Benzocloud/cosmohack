package analysis

import (
	"context"
	"errors"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// NewLegacy adapts the characterization file store while runtime wiring moves
// to the domain persistence port. It is temporary and should disappear with
// service/store after the PostgreSQL cutover.
func NewLegacy(st *store.Store, collector Collector, analyzer Analyzer) *Executor {
	return New(legacyPersistence{store: st}, collector, analyzer)
}

type legacyPersistence struct{ store *store.Store }

var _ Persistence = legacyPersistence{}

func (p legacyPersistence) GetJob(_ context.Context, id string) (domain.Job, error) {
	job, err := p.store.GetJob(id)
	if err != nil {
		return domain.Job{}, mapLegacyError(err)
	}
	return domainJob(*job), nil
}

func (p legacyPersistence) GetArea(_ context.Context, id string) (domain.Area, error) {
	area, err := p.store.GetArea(id)
	if err != nil {
		return domain.Area{}, mapLegacyError(err)
	}
	return domainArea(*area), nil
}

func (p legacyPersistence) SetJobRunning(_ context.Context, id, stage string) error {
	return mapLegacyError(p.store.SetJobRunning(id, stage))
}

func (p legacyPersistence) SetJobStage(_ context.Context, id, stage string) error {
	return mapLegacyError(p.store.SetJobStage(id, stage))
}

func (p legacyPersistence) SetJobFailed(_ context.Context, id, code, message string, retryable bool) error {
	return mapLegacyError(p.store.SetJobFailed(id, store.JobError{Code: code, Message: message, Retryable: retryable}))
}

func (p legacyPersistence) SetJobCancelled(_ context.Context, id string) error {
	return mapLegacyError(p.store.SetJobCancelled(id))
}

func (p legacyPersistence) SetJobInputRevision(_ context.Context, id, revision string) error {
	return mapLegacyError(p.store.SetJobInputRevision(id, revision))
}

func (p legacyPersistence) PutResult(_ context.Context, generation int, jobID string, result domain.AnalysisRecord) error {
	return mapLegacyError(p.store.PutResult(result.AreaID, generation, jobID, storeResult(result)))
}

func (p legacyPersistence) RecoverInterrupted(_ context.Context) error {
	return mapLegacyError(p.store.FailInterrupted())
}

func mapLegacyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrBadState):
		return ErrBadState
	case errors.Is(err, store.ErrGeneration):
		return ErrGeneration
	case errors.Is(err, store.ErrNotFound):
		return ErrNotFound
	default:
		return err
	}
}

func domainArea(area store.Area) domain.Area {
	return domain.Area{
		ID: area.ID, Name: area.Name,
		Geometry: domain.Polygon{Type: area.Geometry.Type, Coordinates: area.Geometry.Coordinates},
		Source:   domain.AreaSource{Kind: area.Source.Kind, ContourID: area.Source.ContourID, Provider: area.Source.Provider},
		Period:   domain.Period{From: area.Period.From, To: area.Period.To}, CreatedAt: area.CreatedAt,
		Generation: area.Generation, ShownResultVersion: area.ShownResultVersion, ActiveJobID: area.ActiveJobID,
	}
}

func domainJob(job store.Job) domain.Job {
	result := domain.Job{
		ID: job.ID, AreaID: job.AreaID, Status: domain.JobStatus(job.Status),
		Period: domain.Period{From: job.Period.From, To: job.Period.To}, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		AreaGeneration: job.AreaGeneration, Stage: job.Stage, ResultVersion: job.ResultVersion, InputRevision: job.InputRevision,
	}
	if job.Error != nil {
		result.ErrorCode = &job.Error.Code
		result.ErrorMessage = &job.Error.Message
		result.ErrorRetryable = &job.Error.Retryable
	}
	return result
}

func storeResult(result domain.AnalysisRecord) store.Result {
	series := make([]store.SeriesPoint, 0, len(result.Series))
	for _, point := range result.Series {
		var interval *store.Period
		if point.Interval != nil {
			interval = &store.Period{From: point.Interval.From, To: point.Interval.To}
		}
		series = append(series, store.SeriesPoint{Date: point.Date, PrimaryNDVI: point.PrimaryNDVI, Value: point.Value,
			State: string(point.State), Method: point.Method, Baseline: point.Baseline, ZScore: point.ZScore,
			Interval: interval, ValidFraction: point.ValidFraction})
	}
	weather := make([]store.WeatherPoint, 0, len(result.Weather))
	for _, point := range result.Weather {
		weather = append(weather, store.WeatherPoint{Date: point.Date, TemperatureMeanC: point.TemperatureMeanC,
			PrecipitationSumMM: point.PrecipitationSumMM, SourceID: point.SourceID})
	}
	events := make([]store.Event, 0, len(result.Events))
	for _, event := range result.Events {
		events = append(events, store.Event{StartDate: event.StartDate, EndDate: event.EndDate, Status: string(event.Status),
			Severity: string(event.Severity), MinZScore: event.MinZScore, EvidenceDates: event.EvidenceDates,
			Facts: event.Facts, Hypothesis: event.Hypothesis, Limitations: event.Limitations})
	}
	var severity *string
	if result.Severity != nil {
		value := string(*result.Severity)
		severity = &value
	}
	return store.Result{ResultVersion: result.ResultVersion, AreaID: result.AreaID, Period: store.Period{From: result.Period.From, To: result.Period.To},
		ComputedAt: result.ComputedAt, SchemaVersion: result.SchemaVersion, FeatureProfile: result.FeatureProfile,
		ModelVersion: result.ModelVersion, Method: result.Method, Status: string(result.Status), Severity: severity,
		Series: series, Weather: weather, Provenance: result.Provenance, Limitations: result.Limitations, Events: events}
}
