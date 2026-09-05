package handler

import (
	"context"
	"errors"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// legacyStorage keeps the existing HTTP characterization tests running while
// the application composition root moves to PostgreSQL.
type legacyStorage struct{ store *store.Store }

func (s legacyStorage) CreateArea(_ context.Context, area domain.Area) error {
	return mapStorageError(s.store.PutArea(storeAreaFromDomain(area)))
}

func (s legacyStorage) UpdateArea(_ context.Context, area domain.Area) error {
	return mapStorageError(s.store.PutArea(storeAreaFromDomain(area)))
}

func (s legacyStorage) GetArea(_ context.Context, id string) (domain.Area, error) {
	area, err := s.store.GetArea(id)
	if err != nil {
		return domain.Area{}, mapStorageError(err)
	}
	out := domainAreaFromStore(*area)
	if out.ShownResultVersion != "" {
		if result, resultErr := s.store.GetResult(id, out.ShownResultVersion); resultErr == nil {
			out.ShownJobID = result.JobID
		}
	}
	return out, nil
}

func (s legacyStorage) ListAreas(_ context.Context) ([]domain.Area, error) {
	areas, err := s.store.ListAreas()
	if err != nil {
		return nil, mapStorageError(err)
	}
	out := make([]domain.Area, 0, len(areas))
	for _, area := range areas {
		mapped := domainAreaFromStore(area)
		if mapped.ShownResultVersion != "" {
			if result, resultErr := s.store.GetResult(area.ID, mapped.ShownResultVersion); resultErr == nil {
				mapped.ShownJobID = result.JobID
			}
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (s legacyStorage) DeleteArea(_ context.Context, id string) ([]string, error) {
	jobs, err := s.store.ListJobsByArea(id)
	if err != nil {
		return nil, mapStorageError(err)
	}
	active := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if job.Status == store.JobQueued || job.Status == store.JobRunning {
			active = append(active, job.ID)
		}
	}
	if err := s.store.DeleteArea(id); err != nil {
		return nil, mapStorageError(err)
	}
	return active, nil
}

func (s legacyStorage) GetJob(_ context.Context, id string) (domain.Job, error) {
	job, err := s.store.GetJob(id)
	if err != nil {
		return domain.Job{}, mapStorageError(err)
	}
	return domainJobFromStore(*job), nil
}

func (s legacyStorage) ListJobsByArea(_ context.Context, areaID string) ([]domain.Job, error) {
	jobs, err := s.store.ListJobsByArea(areaID)
	if err != nil {
		return nil, mapStorageError(err)
	}
	out := make([]domain.Job, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, domainJobFromStore(job))
	}
	return out, nil
}

func (s legacyStorage) PutJobQueued(_ context.Context, job domain.Job) error {
	return mapStorageError(s.store.PutJobQueued(storeJobFromDomain(job)))
}

func (s legacyStorage) DeleteJob(_ context.Context, id string) error {
	return mapStorageError(s.store.DeleteJob(id))
}

func (s legacyStorage) GetResult(_ context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	result, err := s.store.GetResult(areaID, version)
	if err != nil {
		return domain.AnalysisRecord{}, mapStorageError(err)
	}
	return domainResultFromStore(*result), nil
}

func mapStorageError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrBadID):
		return errStorageNotFound
	case errors.Is(err, store.ErrBadState):
		return errStorageBadState
	default:
		return err
	}
}

func domainAreaFromStore(area store.Area) domain.Area {
	return domain.Area{
		ID: area.ID, Name: area.Name,
		Geometry: domain.Polygon{Type: area.Geometry.Type, Coordinates: area.Geometry.Coordinates},
		Source:   domain.AreaSource{Kind: area.Source.Kind, ContourID: area.Source.ContourID, Provider: area.Source.Provider},
		Period:   domain.Period{From: area.Period.From, To: area.Period.To}, CreatedAt: area.CreatedAt,
		Generation: area.Generation, ShownResultVersion: area.ShownResultVersion, ShownJobID: "", ActiveJobID: area.ActiveJobID,
	}
}

func domainJobFromStore(job store.Job) domain.Job {
	out := domain.Job{
		ID: job.ID, AreaID: job.AreaID, Status: domain.JobStatus(job.Status),
		Period: domain.Period{From: job.Period.From, To: job.Period.To}, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		AreaGeneration: job.AreaGeneration, Stage: job.Stage, ResultVersion: job.ResultVersion, InputRevision: job.InputRevision,
	}
	if job.Error != nil {
		out.ErrorCode = &job.Error.Code
		out.ErrorMessage = &job.Error.Message
		out.ErrorRetryable = &job.Error.Retryable
	}
	return out
}

func domainResultFromStore(result store.Result) domain.AnalysisRecord {
	series := make([]domain.SeriesPoint, 0, len(result.Series))
	for _, point := range result.Series {
		var interval *domain.Period
		if point.Interval != nil {
			interval = &domain.Period{From: point.Interval.From, To: point.Interval.To}
		}
		series = append(series, domain.SeriesPoint{Date: point.Date, PrimaryNDVI: point.PrimaryNDVI, Value: point.Value,
			State: domain.PointState(point.State), Method: point.Method, Baseline: point.Baseline, ZScore: point.ZScore,
			Interval: interval, ValidFraction: point.ValidFraction})
	}
	weather := make([]domain.WeatherPoint, 0, len(result.Weather))
	for _, point := range result.Weather {
		weather = append(weather, domain.WeatherPoint{Date: point.Date, TemperatureMeanC: point.TemperatureMeanC,
			PrecipitationSumMM: point.PrecipitationSumMM, SourceID: point.SourceID})
	}
	events := make([]domain.AnomalyEvent, 0, len(result.Events))
	for _, event := range result.Events {
		events = append(events, domain.AnomalyEvent{StartDate: event.StartDate, EndDate: event.EndDate,
			Status: domain.ResultStatus(event.Status), Severity: domain.Severity(event.Severity), MinZScore: event.MinZScore,
			EvidenceDates: event.EvidenceDates, Facts: event.Facts, Hypothesis: event.Hypothesis, Limitations: event.Limitations})
	}
	var severity *domain.Severity
	if result.Severity != nil {
		value := domain.Severity(*result.Severity)
		severity = &value
	}
	return domain.AnalysisRecord{ResultVersion: result.ResultVersion, AreaID: result.AreaID,
		Period: domain.Period{From: result.Period.From, To: result.Period.To}, ComputedAt: result.ComputedAt,
		SchemaVersion: result.SchemaVersion, FeatureProfile: result.FeatureProfile, ModelVersion: result.ModelVersion,
		Method: result.Method, Status: domain.ResultStatus(result.Status), Severity: severity, Series: series,
		Weather: weather, Provenance: result.Provenance, Limitations: result.Limitations, Events: events}
}
