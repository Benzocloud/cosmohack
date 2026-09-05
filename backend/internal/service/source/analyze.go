package source

import (
	"encoding/json"
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
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

// BuildDomain возвращает канонический доменный запрос для сервиса анализа.
// Build оставлен как адаптер совместимости провода на время миграции B1.
func (b *AnalyzeRequestBuilder) BuildDomain(snapshot *Snapshot, requestID string) (*domain.AnalysisRequest, error) {
	wire, err := b.Build(snapshot, requestID)
	if err != nil {
		return nil, err
	}
	var request domain.AnalysisRequest
	if err := json.Unmarshal(wire.body, &request); err != nil {
		return nil, fmt.Errorf("decode canonical analysis request: %w", err)
	}
	return &request, nil
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
		return nil, fmt.Errorf("analysis request has no data snapshot")
	}
	if err := domainsource.RequireIdentifier("request_id", requestID); err != nil {
		return nil, err
	}
	observations := snapshot.observationDTOs()
	if len(observations) == 0 {
		return nil, fmt.Errorf("analysis request has no observations")
	}
	if len(observations) > b.maxObservations {
		return nil, fmt.Errorf("observation count %d exceeds contract limit %d", len(observations), b.maxObservations)
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
	if snapshot.HasMultisensor() {
		payload.SchemaVersion = domain.SchemaVersionV11
		payload.FeatureProfile = domain.FeatureProfileMultisensorV1
	}
	if err := validateSourceReferences(payload); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	maxBodyBytes := b.maxBodyBytes
	if payload.SchemaVersion == domain.SchemaVersionV11 && maxBodyBytes < domain.MaxMultisensorRequestBodyBytes {
		maxBodyBytes = domain.MaxMultisensorRequestBodyBytes
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("request body is %d bytes, contract limit is %d", len(body), maxBodyBytes)
	}
	return &AnalyzeRequest{payload: payload, body: body}, nil
}

func ensureStrictlyAscending(observations []observationDTO) error {
	for index := 1; index < len(observations); index++ {
		if !observations[index-1].Date.Before(observations[index].Date) {
			return fmt.Errorf("observation dates are not strictly ascending: %s after %s",
				observations[index].Date, observations[index-1].Date)
		}
	}
	return nil
}

func validateSourceReferences(payload analyzeRequestPayload) error {
	kinds := make(map[string]domainsource.Kind, len(payload.Sources))
	for _, item := range payload.Sources {
		if _, exists := kinds[item.ID]; exists {
			return fmt.Errorf("source identifier %s is duplicated", item.ID)
		}
		if !item.Kind.Valid() {
			return fmt.Errorf("source %s has kind %q", item.ID, item.Kind)
		}
		kinds[item.ID] = item.Kind
	}
	for _, observation := range payload.Observations {
		if err := requireReference(kinds, observation.NDVISourceID, domainsource.KindSatellite, observation.Date); err != nil {
			return err
		}
		if observation.Weather != nil {
			if err := requireReference(kinds, &observation.Weather.SourceID, domainsource.KindWeather, observation.Date); err != nil {
				return err
			}
		}
		if observation.Reference != nil {
			if err := requireReference(kinds, &observation.Reference.SourceID, domainsource.KindReference, observation.Date); err != nil {
				return err
			}
		}
		if observation.Quality == domainsource.QualityUsable && observation.PrimaryNDVI == nil {
			return fmt.Errorf("observation %s is marked usable without a value", observation.Date)
		}
		if observation.Quality != domainsource.QualityMissing && observation.Interval == nil {
			return fmt.Errorf("observation %s has no aggregation interval", observation.Date)
		}
	}
	return nil
}

func requireReference(kinds map[string]domainsource.Kind, sourceID *string, expected domainsource.Kind, date domainsource.Date) error {
	if sourceID == nil {
		return nil
	}
	kind, exists := kinds[*sourceID]
	if !exists {
		return fmt.Errorf("observation %s references unknown source %s", date, *sourceID)
	}
	if kind != expected {
		return fmt.Errorf("observation %s expects source kind %q, got %q", date, expected, kind)
	}
	return nil
}
