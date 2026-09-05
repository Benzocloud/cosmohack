package source

import (
	"fmt"
	"math"
)

type Quality string

const (
	QualityUsable   Quality = "usable"
	QualityUnusable Quality = "unusable"
	QualityMissing  Quality = "missing"
)

func (q Quality) Valid() bool {
	switch q {
	case QualityUsable, QualityUnusable, QualityMissing:
		return true
	default:
		return false
	}
}

const (
	ReasonNoObservation        = "no_usable_observation"
	ReasonLowValidFraction     = "low_valid_fraction"
	ReasonNoValidSamples       = "no_valid_samples"
	ReasonProviderInterval     = "provider_returned_no_interval"
	ReasonProviderEntryFailed  = "provider_interval_failed"
	ReasonSatelliteUnavailable = "satellite_source_unavailable"
)

type Weather struct {
	sourceID           string
	temperatureMeanC   *float64
	precipitationSumMM *float64
}

func NewWeather(sourceID string, temperatureMeanC, precipitationSumMM *float64) (*Weather, error) {
	if err := requireSourceIdentifier(sourceID); err != nil {
		return nil, err
	}
	if err := requireFiniteOrNil("temperature_mean_c", temperatureMeanC); err != nil {
		return nil, err
	}
	if err := requireFiniteOrNil("precipitation_sum_mm", precipitationSumMM); err != nil {
		return nil, err
	}
	if precipitationSumMM != nil && *precipitationSumMM < 0 {
		return nil, fmt.Errorf("precipitation_sum_mm is negative: %g", *precipitationSumMM)
	}
	return &Weather{
		sourceID:           sourceID,
		temperatureMeanC:   copyFloat(temperatureMeanC),
		precipitationSumMM: copyFloat(precipitationSumMM),
	}, nil
}

func (w *Weather) SourceID() string {
	return w.sourceID
}

func (w *Weather) TemperatureMeanC() *float64 {
	return copyFloat(w.temperatureMeanC)
}

func (w *Weather) PrecipitationSumMM() *float64 {
	return copyFloat(w.precipitationSumMM)
}

type Reference struct {
	sourceID           string
	mean               float64
	std                float64
	referenceYears     int
	targetYearExcluded bool
}

func NewReference(sourceID string, mean, std float64, referenceYears int, targetYearExcluded bool) (*Reference, error) {
	if err := requireSourceIdentifier(sourceID); err != nil {
		return nil, err
	}
	if !isFinite(mean) || !isFinite(std) {
		return nil, fmt.Errorf("seasonal baseline contains a non-finite value")
	}
	if std < 0 {
		return nil, fmt.Errorf("seasonal baseline std is negative: %g", std)
	}
	if referenceYears <= 0 {
		return nil, fmt.Errorf("n_reference_years must be positive, got %d", referenceYears)
	}
	if !targetYearExcluded {
		return nil, fmt.Errorf("seasonal baseline must exclude the target year")
	}
	return &Reference{
		sourceID:           sourceID,
		mean:               mean,
		std:                std,
		referenceYears:     referenceYears,
		targetYearExcluded: true,
	}, nil
}

func (r *Reference) SourceID() string {
	return r.sourceID
}

func (r *Reference) Mean() float64 {
	return r.mean
}

func (r *Reference) Std() float64 {
	return r.std
}

func (r *Reference) ReferenceYears() int {
	return r.referenceYears
}

func (r *Reference) TargetYearExcluded() bool {
	return r.targetYearExcluded
}

type Observation struct {
	date          Date
	primaryNDVI   *float64
	quality       Quality
	ndviSourceID  string
	interval      *DateRange
	validFraction *float64
	missingReason string
	weather       *Weather
	reference     *Reference
}

func (o Observation) Date() Date {
	return o.date
}

func (o Observation) PrimaryNDVI() *float64 {
	return copyFloat(o.primaryNDVI)
}

func (o Observation) Quality() Quality {
	return o.quality
}

func (o Observation) NDVISourceID() string {
	return o.ndviSourceID
}

func (o Observation) Interval() *DateRange {
	if o.interval == nil {
		return nil
	}
	copied := *o.interval
	return &copied
}

func (o Observation) ValidFraction() *float64 {
	return copyFloat(o.validFraction)
}

func (o Observation) MissingReason() string {
	return o.missingReason
}

func (o Observation) Weather() *Weather {
	return o.weather
}

func (o Observation) Reference() *Reference {
	return o.reference
}

type ObservationBuilder struct {
	observation Observation
	failure     error
}

func NewObservationBuilder(date Date) *ObservationBuilder {
	builder := &ObservationBuilder{observation: Observation{date: date, quality: QualityMissing}}
	if date.IsZero() {
		builder.failure = errEmptyDate
	}
	return builder
}

func (b *ObservationBuilder) Measured(ndvi float64, sourceID string, interval DateRange, validFraction *float64) *ObservationBuilder {
	b.observation.primaryNDVI = &ndvi
	b.observation.quality = QualityUsable
	b.observation.ndviSourceID = sourceID
	b.observation.interval = &interval
	b.observation.validFraction = copyFloat(validFraction)
	b.observation.missingReason = ""
	return b
}

func (b *ObservationBuilder) Rejected(ndvi *float64, sourceID string, interval DateRange, reason string, validFraction *float64) *ObservationBuilder {
	b.observation.primaryNDVI = copyFloat(ndvi)
	b.observation.quality = QualityUnusable
	b.observation.ndviSourceID = sourceID
	b.observation.interval = &interval
	b.observation.validFraction = copyFloat(validFraction)
	b.observation.missingReason = reason
	return b
}

func (b *ObservationBuilder) Missing(reason string) *ObservationBuilder {
	b.observation.primaryNDVI = nil
	b.observation.quality = QualityMissing
	b.observation.ndviSourceID = ""
	b.observation.interval = nil
	b.observation.validFraction = nil
	b.observation.missingReason = reason
	return b
}

func (b *ObservationBuilder) WithWeather(weather *Weather) *ObservationBuilder {
	b.observation.weather = weather
	return b
}

func (b *ObservationBuilder) WithReference(reference *Reference) *ObservationBuilder {
	b.observation.reference = reference
	return b
}

func (b *ObservationBuilder) Build() (Observation, error) {
	if b.failure != nil {
		return Observation{}, b.failure
	}
	observation := b.observation
	if !observation.quality.Valid() {
		return Observation{}, fmt.Errorf("quality %q is unsupported", observation.quality)
	}
	if err := requireFiniteOrNil("primary_ndvi", observation.primaryNDVI); err != nil {
		return Observation{}, err
	}
	if err := validateValidFraction(observation.validFraction); err != nil {
		return Observation{}, err
	}
	switch observation.quality {
	case QualityUsable:
		if observation.primaryNDVI == nil {
			return Observation{}, fmt.Errorf("observation %s is marked usable without a value", observation.date)
		}
		if observation.missingReason != "" {
			return Observation{}, fmt.Errorf("observation %s is marked usable with a rejection reason", observation.date)
		}
	case QualityUnusable:
		if observation.missingReason == "" {
			return Observation{}, fmt.Errorf("observation %s is marked unusable without a reason", observation.date)
		}
	case QualityMissing:
		if observation.missingReason == "" {
			return Observation{}, fmt.Errorf("observation %s is marked missing without a reason", observation.date)
		}
		if observation.primaryNDVI != nil || observation.interval != nil || observation.validFraction != nil {
			return Observation{}, fmt.Errorf("observation %s is marked missing but contains measurement data", observation.date)
		}
		if observation.ndviSourceID != "" {
			return Observation{}, fmt.Errorf("observation %s is marked missing with a source reference", observation.date)
		}
	}
	if observation.quality != QualityMissing {
		if err := requireSourceIdentifier(observation.ndviSourceID); err != nil {
			return Observation{}, err
		}
		if observation.interval == nil {
			return Observation{}, fmt.Errorf("observation %s has no aggregation interval", observation.date)
		}
		if !observation.interval.Contains(observation.date) {
			return Observation{}, fmt.Errorf("date %s is outside aggregation interval %s", observation.date, observation.interval)
		}
	}
	return observation, nil
}

func validateValidFraction(fraction *float64) error {
	if fraction == nil {
		return nil
	}
	if err := requireFiniteOrNil("valid_fraction", fraction); err != nil {
		return err
	}
	if *fraction < 0 || *fraction > 1 {
		return fmt.Errorf("valid_fraction %g is outside range [0, 1]", *fraction)
	}
	return nil
}

func requireFiniteOrNil(label string, value *float64) error {
	if value == nil {
		return nil
	}
	if !isFinite(*value) {
		return fmt.Errorf("%s is not a finite number", label)
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func copyFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func Float(value float64) *float64 {
	return &value
}
