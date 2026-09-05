package handler

import (
	"context"
	"errors"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}

type publicArea struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Geometry    domain.Polygon   `json:"geometry"`
	Source      publicAreaSource `json:"source"`
	Period      domain.Period    `json:"period"`
	CreatedAt   string           `json:"created_at"`
	ShownResult *publicShown     `json:"shown_result"`
	ActiveJob   *publicActive    `json:"active_job"`
}

type publicAreaSource struct {
	Kind      string  `json:"kind"`
	ContourID *string `json:"contour_id"`
	Provider  *string `json:"provider"`
}

type publicShown struct {
	ResultVersion string        `json:"result_version"`
	JobID         string        `json:"job_id"`
	Period        domain.Period `json:"period"`
	ComputedAt    string        `json:"computed_at"`
	Status        string        `json:"status"`
	Severity      *string       `json:"severity"`
	ModelVersion  string        `json:"model_version"`
}

type publicActive struct {
	JobID  string  `json:"job_id"`
	Status string  `json:"status"`
	Stage  *string `json:"stage"`
}

type publicJobError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type publicJob struct {
	ID            string          `json:"id"`
	AreaID        string          `json:"area_id"`
	Status        string          `json:"status"`
	Stage         *string         `json:"stage"`
	Period        domain.Period   `json:"period"`
	Error         *publicJobError `json:"error"`
	ResultVersion *string         `json:"result_version"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

func (h *handler) projectArea(ctx context.Context, a domain.Area) (publicArea, error) {
	out := publicArea{ID: a.ID, Name: a.Name, Geometry: a.Geometry,
		Source: publicAreaSource{Kind: a.Source.Kind, ContourID: a.Source.ContourID, Provider: a.Source.Provider},
		Period: a.Period, CreatedAt: formatTime(a.CreatedAt)}
	if a.ShownResultVersion != "" {
		res, err := h.storage.GetResult(ctx, a.ID, a.ShownResultVersion)
		if err != nil && !errors.Is(err, errStorageNotFound) {
			return out, err
		}
		if err == nil {
			jobID := a.ShownJobID
			out.ShownResult = &publicShown{ResultVersion: res.ResultVersion, JobID: jobID, Period: res.Period,
				ComputedAt: formatTime(res.ComputedAt), Status: string(res.Status), Severity: publicSeverity(res.Status, res.Severity), ModelVersion: res.ModelVersion}
		}
	}
	if a.ActiveJobID == "" {
		return out, nil
	}
	j, err := h.storage.GetJob(ctx, a.ActiveJobID)
	if errors.Is(err, errStorageNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if j.Status != domain.JobQueued && j.Status != domain.JobRunning {
		return out, nil
	}
	stage := j.Stage
	if j.Status != domain.JobRunning {
		stage = nil
	}
	out.ActiveJob = &publicActive{JobID: j.ID, Status: string(j.Status), Stage: stage}
	return out, nil
}

func projectJob(j domain.Job) publicJob {
	stage := j.Stage
	if j.Status != domain.JobRunning {
		stage = nil
	}
	var jobErr *publicJobError
	if j.ErrorCode != nil || j.ErrorMessage != nil || j.ErrorRetryable != nil {
		jobErr = &publicJobError{}
		if j.ErrorCode != nil {
			jobErr.Code = *j.ErrorCode
		}
		if j.ErrorMessage != nil {
			jobErr.Message = *j.ErrorMessage
		}
		if j.ErrorRetryable != nil {
			jobErr.Retryable = *j.ErrorRetryable
		}
	}
	return publicJob{ID: j.ID, AreaID: j.AreaID, Status: string(j.Status), Stage: stage, Period: j.Period, Error: jobErr,
		ResultVersion: j.ResultVersion, CreatedAt: formatTime(j.CreatedAt), UpdatedAt: formatTime(j.UpdatedAt)}
}

type publicSeries struct {
	AreaID         string               `json:"area_id"`
	ResultVersion  *string              `json:"result_version"`
	Period         *domain.Period       `json:"period"`
	ComputedAt     *string              `json:"computed_at"`
	SchemaVersion  string               `json:"schema_version,omitempty"`
	FeatureProfile string               `json:"feature_profile,omitempty"`
	ModelVersion   string               `json:"model_version,omitempty"`
	Method         string               `json:"method,omitempty"`
	Status         *string              `json:"status"`
	Severity       *string              `json:"severity"`
	Series         []publicSeriesPoint  `json:"series"`
	Weather        []publicWeatherPoint `json:"weather"`
	Provenance     map[string]any       `json:"provenance,omitempty"`
	Limitations    []string             `json:"limitations"`
}

type publicSeriesPoint struct {
	Date          string            `json:"date"`
	PrimaryNDVI   *float64          `json:"primary_ndvi"`
	Value         *float64          `json:"value"`
	State         domain.PointState `json:"state"`
	Method        *string           `json:"method"`
	Baseline      *float64          `json:"baseline"`
	ZScore        *float64          `json:"z_score"`
	Interval      *domain.Period    `json:"interval"`
	ValidFraction *float64          `json:"valid_fraction"`
}

type publicWeatherPoint struct {
	Date               string   `json:"date"`
	TemperatureMeanC   *float64 `json:"temperature_mean_c"`
	PrecipitationSumMM *float64 `json:"precipitation_sum_mm"`
	SourceID           *string  `json:"source_id"`
}

type publicEvents struct {
	AreaID        string                `json:"area_id"`
	ResultVersion *string               `json:"result_version"`
	Status        *string               `json:"status"`
	Severity      *string               `json:"severity"`
	Events        []domain.AnomalyEvent `json:"events"`
}

func emptySeries(areaID string) publicSeries {
	return publicSeries{AreaID: areaID, Series: []publicSeriesPoint{}, Weather: []publicWeatherPoint{}, Limitations: []string{}}
}

func emptyEvents(areaID string) publicEvents {
	return publicEvents{AreaID: areaID, Events: []domain.AnomalyEvent{}}
}

func projectSeries(res domain.AnalysisRecord) publicSeries {
	ver, status, computedAt, period := res.ResultVersion, string(res.Status), formatTime(res.ComputedAt), res.Period
	series := make([]publicSeriesPoint, 0, len(res.Series))
	for _, point := range res.Series {
		series = append(series, publicSeriesPoint{
			Date: point.Date, PrimaryNDVI: point.PrimaryNDVI, Value: point.Value, State: point.State,
			Method: point.Method, Baseline: point.Baseline, ZScore: point.ZScore,
			Interval: point.Interval, ValidFraction: point.ValidFraction,
		})
	}
	weather := make([]publicWeatherPoint, 0, len(res.Weather))
	for _, point := range res.Weather {
		weather = append(weather, publicWeatherPoint{
			Date: point.Date, TemperatureMeanC: point.TemperatureMeanC,
			PrecipitationSumMM: point.PrecipitationSumMM, SourceID: point.SourceID,
		})
	}
	limitations := res.Limitations
	if limitations == nil {
		limitations = []string{}
	}
	return publicSeries{AreaID: res.AreaID, ResultVersion: &ver, Period: &period, ComputedAt: &computedAt,
		SchemaVersion: res.SchemaVersion, FeatureProfile: res.FeatureProfile, ModelVersion: res.ModelVersion, Method: res.Method,
		Status: &status, Severity: publicSeverity(res.Status, res.Severity), Series: series, Weather: alignWeather(series, res.Weather),
		Provenance: res.Provenance, Limitations: limitations}
}

func publicSeverity(status domain.ResultStatus, stored *domain.Severity) *string {
	switch status {
	case domain.StatusNormal:
		value := string(domain.SeverityNone)
		return &value
	case domain.StatusInsufficientData:
		return nil
	default:
		if stored == nil {
			return nil
		}
		value := string(*stored)
		return &value
	}
}

func alignWeather(series []publicSeriesPoint, weather []domain.WeatherPoint) []publicWeatherPoint {
	byDate := make(map[string]publicWeatherPoint, len(weather))
	for _, point := range weather {
		if point.Date != "" {
			byDate[point.Date] = publicWeatherPoint{
				Date: point.Date, TemperatureMeanC: point.TemperatureMeanC,
				PrecipitationSumMM: point.PrecipitationSumMM, SourceID: point.SourceID,
			}
		}
	}
	out := make([]publicWeatherPoint, 0, len(series))
	for _, point := range series {
		if value, ok := byDate[point.Date]; ok {
			value.Date = point.Date
			out = append(out, value)
			continue
		}
		out = append(out, publicWeatherPoint{Date: point.Date})
	}
	return out
}

func projectEvents(res domain.AnalysisRecord) publicEvents {
	version, status := res.ResultVersion, string(res.Status)
	events := res.Events
	if events == nil {
		events = []domain.AnomalyEvent{}
	}
	return publicEvents{AreaID: res.AreaID, ResultVersion: &version, Status: &status, Severity: publicSeverity(res.Status, res.Severity), Events: events}
}
