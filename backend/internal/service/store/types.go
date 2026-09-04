package store

import "time"

// Period — включительные границы YYYY-MM-DD (UTC).
type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Source — происхождение геометрии участка.
type Source struct {
	Kind      string  `json:"kind"`
	ContourID *string `json:"contour_id"`
	Provider  *string `json:"provider"`
}

// Polygon — GeoJSON Polygon, один ринг.
type Polygon struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

// JobError — причина failed, отдаётся в публичном GET job.
type JobError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Area — снимок участка на диске, включая служебные поля.
type Area struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Geometry           Polygon   `json:"geometry"`
	Source             Source    `json:"source"`
	Period             Period    `json:"period"`
	CreatedAt          time.Time `json:"created_at"`
	Generation         int       `json:"generation"`
	ShownResultVersion string    `json:"shown_result_version"`
	ActiveJobID        string    `json:"active_job_id"`
}

const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// Job — снимок задачи.
type Job struct {
	ID             string    `json:"id"`
	AreaID         string    `json:"area_id"`
	Status         string    `json:"status"`
	Stage          *string   `json:"stage"`
	Period         Period    `json:"period"`
	Error          *JobError `json:"error"`
	ResultVersion  *string   `json:"result_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AreaGeneration int       `json:"area_generation"`
	InputRevision  *string   `json:"input_revision"`
}

// SeriesPoint — точка ряда в сохранённом результате.
type SeriesPoint struct {
	Date          string   `json:"date"`
	PrimaryNDVI   *float64 `json:"primary_ndvi"`
	Value         *float64 `json:"value"`
	State         string   `json:"state"`
	Method        *string  `json:"method"`
	Baseline      *float64 `json:"baseline"`
	ZScore        *float64 `json:"z_score"`
	Interval      *Period  `json:"interval"`
	ValidFraction *float64 `json:"valid_fraction"`
}

// WeatherPoint — погода на дату series.
type WeatherPoint struct {
	Date               string   `json:"date"`
	TemperatureMeanC   *float64 `json:"temperature_mean_c"`
	PrecipitationSumMM *float64 `json:"precipitation_sum_mm"`
	SourceID           *string  `json:"source_id"`
}

// Event — событие аномалии из ответа ML.
type Event struct {
	StartDate     string   `json:"start_date"`
	EndDate       string   `json:"end_date"`
	Status        string   `json:"status"`
	Severity      string   `json:"severity"`
	MinZScore     *float64 `json:"min_z_score"`
	EvidenceDates []string `json:"evidence_dates"`
	Facts         []string `json:"facts"`
	Hypothesis    *string  `json:"hypothesis"`
	Limitations   []string `json:"limitations"`
}

// Result — полный снимок для series/events/shown_result.
type Result struct {
	ResultVersion  string         `json:"result_version"`
	JobID          string         `json:"job_id"`
	AreaID         string         `json:"area_id"`
	Period         Period         `json:"period"`
	ComputedAt     time.Time      `json:"computed_at"`
	SchemaVersion  string         `json:"schema_version"`
	FeatureProfile string         `json:"feature_profile"`
	ModelVersion   string         `json:"model_version"`
	Method         string         `json:"method"`
	Status         string         `json:"status"`
	Severity       *string        `json:"severity"`
	Series         []SeriesPoint  `json:"series"`
	Weather        []WeatherPoint `json:"weather"`
	Provenance     map[string]any `json:"provenance"`
	Limitations    []string       `json:"limitations"`
	Events         []Event        `json:"events"`
}
