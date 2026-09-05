package domain

// Period — включённый диапазон дат YYYY-MM-DD (UTC). Границы включены.
type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Source описывает происхождение данных одного источника в запросе анализа.
// Секреты и URL с токенами сюда не входят; подробную геометрию и сцены
// источник хранит в снимке Go (B1).
type Source struct {
	ID          string     `json:"id"`
	Kind        SourceKind `json:"kind"`
	Provider    string     `json:"provider"`
	Dataset     string     `json:"dataset"`
	Mapping     string     `json:"mapping"`
	RetrievedAt string     `json:"retrieved_at"`
	License     *string    `json:"license"`
}

// Interval — включённый диапазон дат агрегации наблюдения.
type Interval struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Weather — погодный контекст одного дня (реанализ, не измерение на поле).
// Оба значения заданы за день UTC; отсутствие данных кодируется null.
type Weather struct {
	SourceID           string   `json:"source_id"`
	TemperatureMeanC   *float64 `json:"temperature_mean_c"`
	PrecipitationSumMM *float64 `json:"precipitation_sum_mm"`
}

// Reference — проверенный сезонный фон без целевого года. Origin и исключение
// года точки подтверждают B1/ML до передачи; иначе передают null.
type Reference struct {
	SourceID           string  `json:"source_id"`
	Mean               float64 `json:"mean"`
	Std                float64 `json:"std"`
	NReferenceYears    int     `json:"n_reference_years"`
	TargetYearExcluded bool    `json:"target_year_excluded"`
}

// Observation — точка наблюдения запроса. Все поля обязательны; отсутствие
// данных кодируется null, не нулём. Go явно создаёт точки с null на датах,
// которые надо восстановить.
type Observation struct {
	Date          string     `json:"date"`
	PrimaryNDVI   *float64   `json:"primary_ndvi"`
	Quality       Quality    `json:"quality"`
	NDVISourceID  *string    `json:"ndvi_source_id"`
	Interval      *Interval  `json:"interval"`
	ValidFraction *float64   `json:"valid_fraction"`
	MissingReason *string    `json:"missing_reason"`
	Weather       *Weather   `json:"weather"`
	Reference     *Reference `json:"reference"`
}

// AnalysisRequest — тело POST /v1/analyze по контракту v1. Даты наблюдений
// уникальны и строго возрастают; ML возвращает точки только внутри периода.
type AnalysisRequest struct {
	SchemaVersion  string        `json:"schema_version"`
	RequestID      string        `json:"request_id"`
	AreaID         string        `json:"area_id"`
	InputRevision  string        `json:"input_revision"`
	Mode           string        `json:"mode"`
	FeatureProfile string        `json:"feature_profile"`
	AnalysisPeriod Period        `json:"analysis_period"`
	Sources        []Source      `json:"sources"`
	Observations   []Observation `json:"observations"`
}

// SeriesPoint — точка восстановленного ряда результата. PrimaryNDVI повторяет
// исходное значение, включая null; исходное значение не перезаписывается.
type SeriesPoint struct {
	Date          string     `json:"date"`
	PrimaryNDVI   *float64   `json:"primary_ndvi"`
	Value         *float64   `json:"value"`
	State         PointState `json:"state"`
	Method        *string    `json:"method"`
	Baseline      *float64   `json:"baseline"`
	ZScore        *float64   `json:"z_score"`
	Interval      *Period    `json:"interval,omitempty"`
	ValidFraction *float64   `json:"valid_fraction,omitempty"`
}

// AnomalyEvent — негативный период с основаниями. Confirmed требует
// наблюдаемых оснований; собственная импутация их не заменяет.
type AnomalyEvent struct {
	StartDate     string       `json:"start_date"`
	EndDate       string       `json:"end_date"`
	Status        ResultStatus `json:"status"`
	Severity      Severity     `json:"severity"`
	MinZScore     *float64     `json:"min_z_score"`
	EvidenceDates []string     `json:"evidence_dates"`
	Facts         []string     `json:"facts"`
	Hypothesis    *string      `json:"hypothesis"`
	Limitations   []string     `json:"limitations"`
}

// AnalysisResult — успешный ответ POST /v1/analyze. Схема, профиль и модель
// сверяются Go с манифестом выпуска до сохранения результата.
type AnalysisResult struct {
	SchemaVersion  string         `json:"schema_version"`
	RequestID      string         `json:"request_id"`
	AreaID         string         `json:"area_id"`
	InputRevision  string         `json:"input_revision"`
	Mode           string         `json:"mode"`
	FeatureProfile string         `json:"feature_profile"`
	ModelVersion   string         `json:"model_version"`
	Method         string         `json:"method"`
	Status         ResultStatus   `json:"status"`
	Severity       *Severity      `json:"severity"`
	Series         []SeriesPoint  `json:"series"`
	Events         []AnomalyEvent `json:"events"`
	Limitations    []string       `json:"limitations"`
}

// ReadyInfo — тело GET /readyz ML. Готовность не зависит от спутниковых API
// и не означает свободный вычислительный слот.
type ReadyInfo struct {
	Status          string   `json:"status"`
	SchemaVersion   string   `json:"schema_version"`
	FeatureProfiles []string `json:"feature_profiles"`
	ModelVersion    string   `json:"model_version"`
	Reason          *string  `json:"reason"`
}

// MLReadyStatus — значение ReadyInfo.Status готового сервиса.
const MLReadyStatus = "ready"

// MLNotReadyStatus — значение ReadyInfo.Status в теле 503.
const MLNotReadyStatus = "not_ready"
