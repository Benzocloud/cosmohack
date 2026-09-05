package domain

import "time"

// WeatherPoint is the weather context aligned with an analysis series date.
type WeatherPoint struct {
	Date               string   `json:"date"`
	TemperatureMeanC   *float64 `json:"temperature_mean_c"`
	PrecipitationSumMM *float64 `json:"precipitation_sum_mm"`
	SourceID           *string  `json:"source_id,omitempty"`
}

// AnalysisRecord is the domain result produced by analysis before persistence.
// It intentionally has no job identity: one immutable record may be reused by
// several deterministic analysis jobs.
type AnalysisRecord struct {
	ResultVersion  string         `json:"result_version"`
	AreaID         string         `json:"area_id"`
	Period         Period         `json:"period"`
	ComputedAt     time.Time      `json:"computed_at"`
	InputRevision  string         `json:"input_revision"`
	SchemaVersion  string         `json:"schema_version"`
	FeatureProfile string         `json:"feature_profile"`
	ModelVersion   string         `json:"model_version"`
	Method         string         `json:"method"`
	Status         ResultStatus   `json:"status"`
	Severity       *Severity      `json:"severity"`
	Series         []SeriesPoint  `json:"series"`
	Weather        []WeatherPoint `json:"weather"`
	Provenance     map[string]any `json:"provenance"`
	Limitations    []string       `json:"limitations"`
	Events         []AnomalyEvent `json:"events"`
}
