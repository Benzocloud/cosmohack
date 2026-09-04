package source

import (
	"encoding/json"
	"fmt"
)

const (
	SchemaVersion             = "1.0"
	ModeRetrospective         = "retrospective"
	FeatureProfileNDVIWeather = "ndvi-weather-v1"
	DefaultMaxObservations    = 4096
	DefaultMaxBodyBytes       = 1 << 20
)

type analyzeRequestPayload struct {
	SchemaVersion  string           `json:"schema_version"`
	RequestID      string           `json:"request_id"`
	AreaID         string           `json:"area_id"`
	InputRevision  string           `json:"input_revision"`
	Mode           string           `json:"mode"`
	FeatureProfile string           `json:"feature_profile"`
	AnalysisPeriod rangeDTO         `json:"analysis_period"`
	Sources        []sourceDTO      `json:"sources"`
	Observations   []observationDTO `json:"observations"`
}

type AnalyzeRequest struct {
	payload analyzeRequestPayload
	body    []byte
}

func (r *AnalyzeRequest) RequestID() string {
	return r.payload.RequestID
}

func (r *AnalyzeRequest) AreaID() string {
	return r.payload.AreaID
}

func (r *AnalyzeRequest) InputRevision() string {
	return r.payload.InputRevision
}

func (r *AnalyzeRequest) FeatureProfile() string {
	return r.payload.FeatureProfile
}

func (r *AnalyzeRequest) ObservationCount() int {
	return len(r.payload.Observations)
}

func (r *AnalyzeRequest) Body() []byte {
	body := make([]byte, len(r.body))
	copy(body, r.body)
	return body
}

func (r *AnalyzeRequest) MarshalJSON() ([]byte, error) {
	return r.Body(), nil
}

type AnalyzeRequestBuilder struct {
	maxObservations int
	maxBodyBytes    int
}

func NewAnalyzeRequestBuilder(maxObservations, maxBodyBytes int) *AnalyzeRequestBuilder {
	if maxObservations <= 0 {
		maxObservations = DefaultMaxObservations
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	return &AnalyzeRequestBuilder{maxObservations: maxObservations, maxBodyBytes: maxBodyBytes}
}

func (b *AnalyzeRequestBuilder) Build(snapshot *Snapshot, requestID string) (*AnalyzeRequest, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("запрос анализа без снимка данных")
	}
	if err := requireIdentifier("request_id", requestID); err != nil {
		return nil, err
	}
	observations := snapshot.observationDTOs()
	if len(observations) == 0 {
		return nil, fmt.Errorf("запрос анализа без наблюдений")
	}
	if len(observations) > b.maxObservations {
		return nil, fmt.Errorf("наблюдений %d, предел контракта %d", len(observations), b.maxObservations)
	}
	if err := ensureStrictlyAscending(observations); err != nil {
		return nil, err
	}
	payload := analyzeRequestPayload{
		SchemaVersion:  SchemaVersion,
		RequestID:      requestID,
		AreaID:         snapshot.AreaID(),
		InputRevision:  snapshot.Revision(),
		Mode:           ModeRetrospective,
		FeatureProfile: FeatureProfileNDVIWeather,
		AnalysisPeriod: rangeDTO{From: snapshot.Period().From(), To: snapshot.Period().To()},
		Sources:        snapshot.sourceDTOs(),
		Observations:   observations,
	}
	if err := validateSourceReferences(payload); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(body) > b.maxBodyBytes {
		return nil, fmt.Errorf("тело запроса %d байт, предел контракта %d", len(body), b.maxBodyBytes)
	}
	return &AnalyzeRequest{payload: payload, body: body}, nil
}

func ensureStrictlyAscending(observations []observationDTO) error {
	for index := 1; index < len(observations); index++ {
		if !observations[index-1].Date.Before(observations[index].Date) {
			return fmt.Errorf("даты наблюдений не строго возрастают: %s после %s",
				observations[index].Date, observations[index-1].Date)
		}
	}
	return nil
}

func validateSourceReferences(payload analyzeRequestPayload) error {
	kinds := make(map[string]Kind, len(payload.Sources))
	for _, item := range payload.Sources {
		if _, exists := kinds[item.ID]; exists {
			return fmt.Errorf("идентификатор источника %s повторяется", item.ID)
		}
		if !item.Kind.Valid() {
			return fmt.Errorf("источник %s имеет вид %q", item.ID, item.Kind)
		}
		kinds[item.ID] = item.Kind
	}
	for _, observation := range payload.Observations {
		if err := requireReference(kinds, observation.NDVISourceID, KindSatellite, observation.Date); err != nil {
			return err
		}
		if observation.Weather != nil {
			if err := requireReference(kinds, &observation.Weather.SourceID, KindWeather, observation.Date); err != nil {
				return err
			}
		}
		if observation.Reference != nil {
			if err := requireReference(kinds, &observation.Reference.SourceID, KindReference, observation.Date); err != nil {
				return err
			}
		}
		if observation.Quality == QualityUsable && observation.PrimaryNDVI == nil {
			return fmt.Errorf("наблюдение %s помечено usable без значения", observation.Date)
		}
		if observation.Quality != QualityMissing && observation.Interval == nil {
			return fmt.Errorf("наблюдение %s без интервала агрегации", observation.Date)
		}
	}
	return nil
}

func requireReference(kinds map[string]Kind, sourceID *string, expected Kind, date Date) error {
	if sourceID == nil {
		return nil
	}
	kind, exists := kinds[*sourceID]
	if !exists {
		return fmt.Errorf("наблюдение %s ссылается на неизвестный источник %s", date, *sourceID)
	}
	if kind != expected {
		return fmt.Errorf("наблюдение %s ожидает источник вида %q, найден %q", date, expected, kind)
	}
	return nil
}
