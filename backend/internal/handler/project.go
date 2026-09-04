package handler

import (
	"errors"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}

type publicArea struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Geometry    store.Polygon `json:"geometry"`
	Source      store.Source  `json:"source"`
	Period      store.Period  `json:"period"`
	CreatedAt   string        `json:"created_at"`
	ShownResult *publicShown  `json:"shown_result"`
	ActiveJob   *publicActive `json:"active_job"`
}

type publicShown struct {
	ResultVersion string       `json:"result_version"`
	JobID         string       `json:"job_id"`
	Period        store.Period `json:"period"`
	ComputedAt    string       `json:"computed_at"`
	Status        string       `json:"status"`
	Severity      *string      `json:"severity"`
	ModelVersion  string       `json:"model_version"`
}

type publicActive struct {
	JobID  string  `json:"job_id"`
	Status string  `json:"status"`
	Stage  *string `json:"stage"`
}

type publicJob struct {
	ID            string          `json:"id"`
	AreaID        string          `json:"area_id"`
	Status        string          `json:"status"`
	Stage         *string         `json:"stage"`
	Period        store.Period    `json:"period"`
	Error         *store.JobError `json:"error"`
	ResultVersion *string         `json:"result_version"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

func (h *handler) projectArea(a store.Area) (publicArea, error) {
	out := publicArea{
		ID:        a.ID,
		Name:      a.Name,
		Geometry:  a.Geometry,
		Source:    a.Source,
		Period:    a.Period,
		CreatedAt: formatTime(a.CreatedAt),
	}
	if a.ShownResultVersion != "" {
		res, err := h.store.GetResult(a.ID, a.ShownResultVersion)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return out, err
		}
		if err == nil {
			out.ShownResult = &publicShown{
				ResultVersion: res.ResultVersion,
				JobID:         res.JobID,
				Period:        res.Period,
				ComputedAt:    formatTime(res.ComputedAt),
				Status:        res.Status,
				Severity:      publicSeverity(res.Status, res.Severity),
				ModelVersion:  res.ModelVersion,
			}
		}
	}
	if a.ActiveJobID == "" {
		return out, nil
	}
	j, err := h.store.GetJob(a.ActiveJobID)
	if errors.Is(err, store.ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if j.Status != store.JobQueued && j.Status != store.JobRunning {
		return out, nil
	}
	stage := j.Stage
	if j.Status != store.JobRunning {
		stage = nil
	}
	out.ActiveJob = &publicActive{JobID: j.ID, Status: j.Status, Stage: stage}
	return out, nil
}

func projectJob(j store.Job) publicJob {
	stage := j.Stage
	if j.Status != store.JobRunning {
		stage = nil
	}
	return publicJob{
		ID:            j.ID,
		AreaID:        j.AreaID,
		Status:        j.Status,
		Stage:         stage,
		Period:        j.Period,
		Error:         j.Error,
		ResultVersion: j.ResultVersion,
		CreatedAt:     formatTime(j.CreatedAt),
		UpdatedAt:     formatTime(j.UpdatedAt),
	}
}

type publicSeries struct {
	AreaID         string               `json:"area_id"`
	ResultVersion  *string              `json:"result_version"`
	Period         *store.Period        `json:"period"`
	ComputedAt     *string              `json:"computed_at"`
	SchemaVersion  string               `json:"schema_version,omitempty"`
	FeatureProfile string               `json:"feature_profile,omitempty"`
	ModelVersion   string               `json:"model_version,omitempty"`
	Method         string               `json:"method,omitempty"`
	Status         *string              `json:"status"`
	Severity       *string              `json:"severity"`
	Series         []store.SeriesPoint  `json:"series"`
	Weather        []store.WeatherPoint `json:"weather"`
	Provenance     map[string]any       `json:"provenance,omitempty"`
	Limitations    []string             `json:"limitations"`
}

type publicEvents struct {
	AreaID        string        `json:"area_id"`
	ResultVersion *string       `json:"result_version"`
	Status        *string       `json:"status"`
	Severity      *string       `json:"severity"`
	Events        []store.Event `json:"events"`
}

func emptySeries(areaID string) publicSeries {
	return publicSeries{
		AreaID:      areaID,
		Series:      []store.SeriesPoint{},
		Weather:     []store.WeatherPoint{},
		Limitations: []string{},
	}
}

func emptyEvents(areaID string) publicEvents {
	return publicEvents{AreaID: areaID, Events: []store.Event{}}
}

func projectSeries(res store.Result) publicSeries {
	ver := res.ResultVersion
	st := res.Status
	comp := formatTime(res.ComputedAt)
	period := res.Period
	series := res.Series
	if series == nil {
		series = []store.SeriesPoint{}
	}
	weather := res.Weather
	if weather == nil {
		weather = []store.WeatherPoint{}
	}
	lim := res.Limitations
	if lim == nil {
		lim = []string{}
	}
	return publicSeries{
		AreaID:         res.AreaID,
		ResultVersion:  &ver,
		Period:         &period,
		ComputedAt:     &comp,
		SchemaVersion:  res.SchemaVersion,
		FeatureProfile: res.FeatureProfile,
		ModelVersion:   res.ModelVersion,
		Method:         res.Method,
		Status:         &st,
		Severity:       publicSeverity(res.Status, res.Severity),
		Series:         series,
		Weather:        alignWeather(series, weather),
		Provenance:     res.Provenance,
		Limitations:    lim,
	}
}

// publicSeverity: нет результата обрабатывается снаружи; normal → "none"; insufficient_data → null.
func publicSeverity(status string, stored *string) *string {
	switch status {
	case "normal":
		s := "none"
		return &s
	case "insufficient_data":
		return nil
	default:
		return stored
	}
}

// alignWeather: одна точка на каждую дату series; нет данных — null, не 0 и не пропуск даты.
func alignWeather(series []store.SeriesPoint, weather []store.WeatherPoint) []store.WeatherPoint {
	byDate := make(map[string]store.WeatherPoint, len(weather))
	for _, w := range weather {
		if w.Date == "" {
			continue
		}
		byDate[w.Date] = w
	}
	out := make([]store.WeatherPoint, 0, len(series))
	for _, p := range series {
		if w, ok := byDate[p.Date]; ok {
			w.Date = p.Date
			out = append(out, w)
			continue
		}
		out = append(out, store.WeatherPoint{Date: p.Date})
	}
	return out
}

func projectEvents(res store.Result) publicEvents {
	ver := res.ResultVersion
	st := res.Status
	ev := res.Events
	if ev == nil {
		ev = []store.Event{}
	}
	return publicEvents{
		AreaID:        res.AreaID,
		ResultVersion: &ver,
		Status:        &st,
		Severity:      publicSeverity(res.Status, res.Severity),
		Events:        ev,
	}
}
