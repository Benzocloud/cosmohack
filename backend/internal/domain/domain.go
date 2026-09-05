// Package domain содержит общие типы и константы всех backend-модулей:
// HTTP-контракт Go ↔ ML, участки, задания и манифест выпуска.
//
// В пакете нет ввода-вывода, провайдеров, хранилища, HTTP-обработчиков и
// алгоритмов: source, repository, api, jobs и mlbridge не должны образовывать
// цикл импортов. Изменение типов согласуется с затронутыми владельцами
// (см. .agent/instructions.md), за общий diff отвечает B4.
package domain

// Версия HTTP-контракта Go ↔ ML. Обе стороны проверяют точное совпадение.
const SchemaVersionV1 = "1.0"

// Режим анализа MVP: восстановление по известному контексту до и после пропуска.
const ModeRetrospective = "retrospective"

// Начальный профиль признаков веб-входа: NDVI плюс погода.
const FeatureProfileNDVIWeatherV1 = "ndvi-weather-v1"

// SourceKind — тип источника данных в запросе анализа.
type SourceKind string

const (
	SourceSatellite SourceKind = "satellite"
	SourceWeather   SourceKind = "weather"
	SourceReference SourceKind = "reference"
)

// Quality — класс наблюдения точки. Отсутствие значения кодируется
// QualityMissing, а не нулём.
type Quality string

const (
	QualityUsable   Quality = "usable"
	QualityUnusable Quality = "unusable"
	QualityMissing  Quality = "missing"
)

// PointState — происхождение значения в точке результата. Восстановленное
// значение никогда не помечается как спутниковое измерение.
type PointState string

const (
	StateObserved PointState = "observed"
	StateImputed  PointState = "imputed"
	StateMissing  PointState = "missing"
)

// ResultStatus — сводный статус вывода по анализируемому периоду.
// Статус не является вероятностью и не доказывает причину события.
type ResultStatus string

const (
	StatusNormal           ResultStatus = "normal"
	StatusCandidate        ResultStatus = "candidate"
	StatusConfirmed        ResultStatus = "confirmed"
	StatusInsufficientData ResultStatus = "insufficient_data"
)

// Severity — тяжесть отклонения. При недостатке данных Severity отсутствует
// (null), при normal — SeverityNone.
type Severity string

const (
	SeverityNone     Severity = "none"
	SeverityModerate Severity = "moderate"
	SeverityHigh     Severity = "high"
)

// JobStatus — состояние задачи в исполнителе Go. Переходы сообщает
// исполнитель, хранит B3.
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// Причина прерывания незавершённых задач после рестарта Go.
const InterruptReason = "interrupted"

// Коды ошибок взаимодействия с ML в задаче Go. Коды берутся из таблицы
// обработки ошибок HTTP-контракта (.agent/contracts/go-ml-http.md).
type MLErrorCode string

const (
	MLErrorInvalidRequest   MLErrorCode = "ml_invalid_request"
	MLErrorInputTooLarge    MLErrorCode = "ml_input_too_large"
	MLErrorContractMismatch MLErrorCode = "ml_contract_mismatch"
	MLErrorBusy             MLErrorCode = "ml_busy"
	MLErrorUnavailable      MLErrorCode = "ml_unavailable"
	MLErrorTimeout          MLErrorCode = "ml_timeout"
	MLErrorInvalidResponse  MLErrorCode = "ml_invalid_response"
	MLErrorInternal         MLErrorCode = "ml_internal_error"
)

// Начальные лимиты HTTP-контракта v1. Значения стартовые, изменения
// согласуют B4 и ML и фиксируют в README.
const (
	// MaxObservationsPerRequest — максимум точек наблюдения с историческим контекстом.
	MaxObservationsPerRequest = 4096
	// MaxRequestBodyBytes — предел тела запроса (1 MiB).
	MaxRequestBodyBytes = 1 << 20
	// MaxResponseBodyBytes — предел тела ответа (4 MiB).
	MaxResponseBodyBytes = 4 << 20
	// MaxIDLength — предел длины идентификаторов и версий.
	MaxIDLength = 128
)
