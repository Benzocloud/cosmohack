package domain

// Polygon — геометрия GeoJSON Polygon в WGS84 с порядком longitude/latitude.
// Внешний контур и дыры; каждая точка — [lon, lat].
type Polygon struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

// Area — участок пользователя или найденный контур. Происхождение контура
// (OSM Overpass, рисунок пользователя) сохраняется в ContourSource.
type Area struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ContourSource string  `json:"contour_source"`
	Geometry      Polygon `json:"geometry"`
	Period        Period  `json:"period"`
}

// Job — задача анализа. Состояния и результаты принадлежат Go; хранит B3.
// ErrorCode/ErrorMessage заполняются при failed кодами MLErrorCode,
// ResultVersion появляется после сохранения результата.
type Job struct {
	ID            string    `json:"id"`
	AreaID        string    `json:"area_id"`
	Status        JobStatus `json:"status"`
	Stage         *string   `json:"stage"`
	ErrorCode     *string   `json:"error_code"`
	ErrorMessage  *string   `json:"error_message"`
	ResultVersion *string   `json:"result_version"`
}

// Терминальные стадии прогресса исполнителя. B1 и ML могут добавлять свои
// подробности; пустые значения не подставляются.
const (
	StageCollectSatellite = "collect_satellite"
	StageCollectWeather   = "collect_weather"
	StagePrepareInput     = "prepare_input"
	StageAnalyze          = "analyze"
	StageSaveResult       = "save_result"
)

// ReleaseManifest связывает пару immutable-образов выпуска с версиями
// контракта, профиля признаков и модели. Хранится деплоем и проверяется Go
// перед анализом.
type ReleaseManifest struct {
	GoImageDigest  string `json:"go_image_digest"`
	MLImageDigest  string `json:"ml_image_digest"`
	SchemaVersion  string `json:"schema_version"`
	FeatureProfile string `json:"feature_profile"`
	ModelVersion   string `json:"model_version"`
}
