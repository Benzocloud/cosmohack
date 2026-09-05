package cdse

import (
	"fmt"
	"math"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

// Indices содержит средние значения индексов за один интервал CDSE.
// Nil означает, что для индекса не осталось пригодных пикселей.
// NDWI использует определение Gao: (B08-B11)/(B08+B11).
type Indices struct {
	NDVI *float64
	EVI  *float64
	NDWI *float64
}

// IndexSample — локальное представление CDSE, пока контракт Go v1.1 не
// передаёт индексы отдельных сенсоров через общий source domain.
type IndexSample struct {
	date          source.Date
	interval      source.DateRange
	indices       Indices
	validFraction *float64
	usable        bool
	reason        string
}

func (s IndexSample) Date() source.Date {
	return s.date
}

func (s IndexSample) Interval() source.DateRange {
	return s.interval
}

func (s IndexSample) Indices() Indices {
	return Indices{
		NDVI: copyIndexValue(s.indices.NDVI),
		EVI:  copyIndexValue(s.indices.EVI),
		NDWI: copyIndexValue(s.indices.NDWI),
	}
}

func (s IndexSample) ValidFraction() *float64 {
	return copyIndexValue(s.validFraction)
}

func (s IndexSample) Usable() bool {
	return s.usable
}

func (s IndexSample) Reason() string {
	return s.reason
}

// IndexSeries — локальный ряд индексов из CDSE Statistical API.
type IndexSeries struct {
	descriptor source.Descriptor
	samples    []IndexSample
	notes      []string
}

func NewIndexSeries(descriptor source.Descriptor, samples []IndexSample, notes []string) IndexSeries {
	storedSamples := make([]IndexSample, len(samples))
	copy(storedSamples, samples)
	storedNotes := make([]string, len(notes))
	copy(storedNotes, notes)
	return IndexSeries{descriptor: descriptor, samples: storedSamples, notes: storedNotes}
}

func (s IndexSeries) Descriptor() source.Descriptor {
	return s.descriptor
}

func (s IndexSeries) Samples() []IndexSample {
	samples := make([]IndexSample, len(s.samples))
	copy(samples, s.samples)
	return samples
}

func (s IndexSeries) Notes() []string {
	notes := make([]string, len(s.notes))
	copy(notes, s.notes)
	return notes
}

func newIndexSample(
	interval source.DateRange,
	indices Indices,
	validFraction *float64,
	usable bool,
	reason string,
) (IndexSample, error) {
	if interval.IsZero() {
		return IndexSample{}, fmt.Errorf("index sample has no aggregation interval")
	}
	invalidFraction := validFraction != nil &&
		(*validFraction < 0 || *validFraction > 1 ||
			math.IsNaN(*validFraction) || math.IsInf(*validFraction, 0))
	if invalidFraction {
		return IndexSample{}, fmt.Errorf("index sample valid fraction is outside range [0, 1]")
	}
	for name, value := range map[string]*float64{
		"ndvi": indices.NDVI,
		"evi":  indices.EVI,
		"ndwi": indices.NDWI,
	} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return IndexSample{}, fmt.Errorf("index sample %s is not finite", name)
		}
	}
	if usable && indices.NDVI == nil {
		return IndexSample{}, fmt.Errorf("usable index sample has no NDVI")
	}
	if !usable && reason == "" {
		return IndexSample{}, fmt.Errorf("unusable index sample has no reason")
	}
	return IndexSample{
		date:          indexIntervalMidpoint(interval),
		interval:      interval,
		indices:       indices,
		validFraction: copyIndexValue(validFraction),
		usable:        usable,
		reason:        reason,
	}, nil
}

func indexIntervalMidpoint(interval source.DateRange) source.Date {
	return interval.From().AddDays((interval.Days() - 1) / 2)
}

func copyIndexValue(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
