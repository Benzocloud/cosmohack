package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

const revisionPrefix = "snap-"

type SnapshotSpec struct {
	AreaID       string
	Period       domainsource.DateRange
	Polygon      *geom.Polygon
	Descriptors  []domainsource.Descriptor
	Observations []domainsource.Observation
	WeatherCell  *geom.Coordinate
	Limitations  []string
	CollectedAt  time.Time
}

type Snapshot struct {
	areaID       string
	period       domainsource.DateRange
	polygon      *geom.Polygon
	descriptors  []domainsource.Descriptor
	observations []domainsource.Observation
	weatherCell  *geom.Coordinate
	limitations  []string
	collectedAt  time.Time
	revision     string
}

func NewSnapshot(spec SnapshotSpec) (*Snapshot, error) {
	if err := domainsource.RequireIdentifier("area_id", spec.AreaID); err != nil {
		return nil, err
	}
	if spec.Period.IsZero() {
		return nil, fmt.Errorf("snapshot %s has no period", spec.AreaID)
	}
	if spec.Polygon == nil {
		return nil, fmt.Errorf("snapshot %s has no geometry", spec.AreaID)
	}
	if spec.CollectedAt.IsZero() {
		return nil, fmt.Errorf("snapshot %s has no collection time", spec.AreaID)
	}
	index, err := indexDescriptors(spec.Descriptors)
	if err != nil {
		return nil, err
	}
	if err := validateObservations(spec.Period, spec.Observations, index); err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		areaID:       spec.AreaID,
		period:       spec.Period,
		polygon:      spec.Polygon,
		descriptors:  append([]domainsource.Descriptor(nil), spec.Descriptors...),
		observations: append([]domainsource.Observation(nil), spec.Observations...),
		weatherCell:  spec.WeatherCell,
		limitations:  append([]string(nil), spec.Limitations...),
		collectedAt:  spec.CollectedAt.UTC(),
	}
	revision, err := snapshot.computeRevision()
	if err != nil {
		return nil, err
	}
	snapshot.revision = revision
	return snapshot, nil
}

func (s *Snapshot) AreaID() string {
	return s.areaID
}

func (s *Snapshot) Revision() string {
	return s.revision
}

func (s *Snapshot) Period() domainsource.DateRange {
	return s.period
}

func (s *Snapshot) Polygon() *geom.Polygon {
	return s.polygon
}

func (s *Snapshot) Descriptors() []domainsource.Descriptor {
	descriptors := make([]domainsource.Descriptor, len(s.descriptors))
	copy(descriptors, s.descriptors)
	return descriptors
}

func (s *Snapshot) Observations() []domainsource.Observation {
	observations := make([]domainsource.Observation, len(s.observations))
	copy(observations, s.observations)
	return observations
}

func (s *Snapshot) WeatherCell() *geom.Coordinate {
	if s.weatherCell == nil {
		return nil
	}
	cell := *s.weatherCell
	return &cell
}

func (s *Snapshot) Limitations() []string {
	limitations := make([]string, len(s.limitations))
	copy(limitations, s.limitations)
	return limitations
}

func (s *Snapshot) CollectedAt() time.Time {
	return s.collectedAt
}

func (s *Snapshot) UsableCount() int {
	return countUsable(s.observations)
}

func (s *Snapshot) HasMultisensor() bool {
	for _, observation := range s.observations {
		if observation.Indices() != nil {
			return true
		}
	}
	return false
}

func (s *Snapshot) MarshalJSON() ([]byte, error) {
	geometry, err := geom.NewPolygonCodec(0).Encode(s.polygon)
	if err != nil {
		return nil, err
	}
	var cell *pointDTO
	if s.weatherCell != nil {
		cell = &pointDTO{Longitude: s.weatherCell.Lon(), Latitude: s.weatherCell.Lat()}
	}
	representative := s.polygon.RepresentativePoint()
	return json.Marshal(struct {
		AreaID              string           `json:"area_id"`
		InputRevision       string           `json:"input_revision"`
		CollectedAt         string           `json:"collected_at"`
		Period              rangeDTO         `json:"period"`
		Geometry            json.RawMessage  `json:"geometry"`
		AreaHectares        float64          `json:"area_hectares"`
		RepresentativePoint pointDTO         `json:"representative_point"`
		WeatherCell         *pointDTO        `json:"weather_cell"`
		Sources             []sourceDTO      `json:"sources"`
		Observations        []observationDTO `json:"observations"`
		Limitations         []string         `json:"limitations"`
	}{
		AreaID:              s.areaID,
		InputRevision:       s.revision,
		CollectedAt:         s.collectedAt.Format(time.RFC3339),
		Period:              rangeDTO{From: s.period.From(), To: s.period.To()},
		Geometry:            geometry,
		AreaHectares:        s.polygon.AreaHectares(),
		RepresentativePoint: pointDTO{Longitude: representative.Lon(), Latitude: representative.Lat()},
		WeatherCell:         cell,
		Sources:             s.sourceDTOs(),
		Observations:        s.observationDTOs(),
		Limitations:         s.Limitations(),
	})
}

func (s *Snapshot) sourceDTOs() []sourceDTO {
	sources := make([]sourceDTO, 0, len(s.descriptors))
	for _, descriptor := range s.descriptors {
		sources = append(sources, newSourceDTO(descriptor))
	}
	return sources
}

func (s *Snapshot) observationDTOs() []observationDTO {
	observations := make([]observationDTO, 0, len(s.observations))
	for _, observation := range s.observations {
		observations = append(observations, newObservationDTO(observation))
	}
	return observations
}

func (s *Snapshot) computeRevision() (string, error) {
	geometry, err := geom.NewPolygonCodec(0).Encode(s.polygon)
	if err != nil {
		return "", err
	}
	sources := make([]sourceDTO, 0, len(s.descriptors))
	for _, descriptor := range s.descriptors {
		dto := newSourceDTO(descriptor)
		dto.RetrievedAt = ""
		sources = append(sources, dto)
	}
	payload, err := json.Marshal(struct {
		AreaID       string           `json:"area_id"`
		Period       rangeDTO         `json:"period"`
		Geometry     json.RawMessage  `json:"geometry"`
		Sources      []sourceDTO      `json:"sources"`
		Observations []observationDTO `json:"observations"`
	}{
		AreaID:       s.areaID,
		Period:       rangeDTO{From: s.period.From(), To: s.period.To()},
		Geometry:     geometry,
		Sources:      sources,
		Observations: s.observationDTOs(),
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return revisionPrefix + hex.EncodeToString(digest[:])[:24], nil
}

func indexDescriptors(descriptors []domainsource.Descriptor) (map[string]domainsource.Descriptor, error) {
	index := make(map[string]domainsource.Descriptor, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.IsZero() {
			return nil, fmt.Errorf("source has no identifier")
		}
		if _, exists := index[descriptor.ID()]; exists {
			return nil, fmt.Errorf("source identifier %s is duplicated", descriptor.ID())
		}
		index[descriptor.ID()] = descriptor
	}
	return index, nil
}

func validateObservations(period domainsource.DateRange, observations []domainsource.Observation, index map[string]domainsource.Descriptor) error {
	expected := period.Dates()
	if len(observations) != len(expected) {
		return fmt.Errorf("observation count %d does not match period length %d", len(observations), len(expected))
	}
	for position, observation := range observations {
		if !observation.Date().Equal(expected[position]) {
			return fmt.Errorf("observation at position %d has date %s, expected %s", position, observation.Date(), expected[position])
		}
		if sourceID := observation.NDVISourceID(); sourceID != "" {
			descriptor, exists := index[sourceID]
			if !exists {
				return fmt.Errorf("observation %s references unknown source %s", observation.Date(), sourceID)
			}
			if descriptor.Kind() != domainsource.KindSatellite {
				return fmt.Errorf("observation %s references source of kind %q", observation.Date(), descriptor.Kind())
			}
		}
		if weather := observation.Weather(); weather != nil {
			descriptor, exists := index[weather.SourceID()]
			if !exists {
				return fmt.Errorf("weather on %s references unknown source %s", observation.Date(), weather.SourceID())
			}
			if descriptor.Kind() != domainsource.KindWeather {
				return fmt.Errorf("weather on %s references source of kind %q", observation.Date(), descriptor.Kind())
			}
		}
		if reference := observation.Reference(); reference != nil {
			descriptor, exists := index[reference.SourceID()]
			if !exists {
				return fmt.Errorf("reference on %s points to unknown source %s", observation.Date(), reference.SourceID())
			}
			if descriptor.Kind() != domainsource.KindReference {
				return fmt.Errorf("reference on %s points to source of kind %q", observation.Date(), descriptor.Kind())
			}
		}
	}
	return nil
}
