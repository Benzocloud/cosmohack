package source

import (
	"context"
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

type ContourFinder interface {
	FindContours(ctx context.Context, bbox geom.BBox) (ContourSearchResult, error)
}

type SatelliteRequest struct {
	polygon *geom.Polygon
	period  DateRange
}

func NewSatelliteRequest(polygon *geom.Polygon, period DateRange) (SatelliteRequest, error) {
	if polygon == nil {
		return SatelliteRequest{}, fmt.Errorf("satellite data request has no geometry")
	}
	if period.IsZero() {
		return SatelliteRequest{}, fmt.Errorf("satellite data request has no period")
	}
	return SatelliteRequest{polygon: polygon, period: period}, nil
}

func (r SatelliteRequest) Polygon() *geom.Polygon {
	return r.polygon
}

func (r SatelliteRequest) Period() DateRange {
	return r.period
}

type SatelliteSample struct {
	date          Date
	interval      DateRange
	ndvi          *float64
	indices       *SatelliteIndices
	validFraction *float64
	usable        bool
	reason        string
}

func NewSatelliteSample(interval DateRange, ndvi, validFraction *float64, usable bool, reason string) (SatelliteSample, error) {
	if interval.IsZero() {
		return SatelliteSample{}, fmt.Errorf("satellite sample has no aggregation interval")
	}
	if err := requireFiniteOrNil("ndvi", ndvi); err != nil {
		return SatelliteSample{}, err
	}
	if err := validateValidFraction(validFraction); err != nil {
		return SatelliteSample{}, err
	}
	if usable && ndvi == nil {
		return SatelliteSample{}, fmt.Errorf("usable satellite sample %s has no value", interval)
	}
	if !usable && reason == "" {
		return SatelliteSample{}, fmt.Errorf("unusable satellite sample %s has no reason", interval)
	}
	return SatelliteSample{
		date:          intervalMidpoint(interval),
		interval:      interval,
		ndvi:          copyFloat(ndvi),
		validFraction: copyFloat(validFraction),
		usable:        usable,
		reason:        reason,
	}, nil
}

// WithIndices добавляет к образцу мультисенсорные значения, рассчитанные провайдером.
func (s SatelliteSample) WithIndices(indices *SatelliteIndices) (SatelliteSample, error) {
	if err := validateSatelliteIndices(indices); err != nil {
		return SatelliteSample{}, err
	}
	s.indices = nil
	if indices != nil {
		cloned := indices.Values()
		s.indices = &cloned
	}
	return s, nil
}

func (s SatelliteSample) Date() Date {
	return s.date
}

func (s SatelliteSample) Interval() DateRange {
	return s.interval
}

func (s SatelliteSample) NDVI() *float64 {
	return copyFloat(s.ndvi)
}

func (s SatelliteSample) ValidFraction() *float64 {
	return copyFloat(s.validFraction)
}

func (s SatelliteSample) Indices() *SatelliteIndices {
	if s.indices == nil {
		return nil
	}
	cloned := s.indices.Values()
	return &cloned
}

func (s SatelliteSample) Usable() bool {
	return s.usable
}

func (s SatelliteSample) Reason() string {
	return s.reason
}

type SatelliteSeries struct {
	descriptor Descriptor
	samples    []SatelliteSample
	notes      []string
}

func NewSatelliteSeries(descriptor Descriptor, samples []SatelliteSample, notes []string) (SatelliteSeries, error) {
	if descriptor.Kind() != KindSatellite {
		return SatelliteSeries{}, fmt.Errorf("satellite data source has kind %q", descriptor.Kind())
	}
	stored := make([]SatelliteSample, len(samples))
	copy(stored, samples)
	storedNotes := make([]string, len(notes))
	copy(storedNotes, notes)
	return SatelliteSeries{descriptor: descriptor, samples: stored, notes: storedNotes}, nil
}

func (s SatelliteSeries) Descriptor() Descriptor {
	return s.descriptor
}

func (s SatelliteSeries) Samples() []SatelliteSample {
	samples := make([]SatelliteSample, len(s.samples))
	copy(samples, s.samples)
	return samples
}

func (s SatelliteSeries) Notes() []string {
	notes := make([]string, len(s.notes))
	copy(notes, s.notes)
	return notes
}

type SatelliteProvider interface {
	FetchNDVI(ctx context.Context, request SatelliteRequest) (SatelliteSeries, error)
}

type WeatherRequest struct {
	point  geom.Coordinate
	period DateRange
}

func NewWeatherRequest(point geom.Coordinate, period DateRange) (WeatherRequest, error) {
	if period.IsZero() {
		return WeatherRequest{}, fmt.Errorf("weather request has no period")
	}
	return WeatherRequest{point: point, period: period}, nil
}

func (r WeatherRequest) Point() geom.Coordinate {
	return r.point
}

func (r WeatherRequest) Period() DateRange {
	return r.period
}

type WeatherDay struct {
	date               Date
	temperatureMeanC   *float64
	precipitationSumMM *float64
}

func NewWeatherDay(date Date, temperatureMeanC, precipitationSumMM *float64) (WeatherDay, error) {
	if date.IsZero() {
		return WeatherDay{}, errEmptyDate
	}
	if err := requireFiniteOrNil("temperature_mean_c", temperatureMeanC); err != nil {
		return WeatherDay{}, err
	}
	if err := requireFiniteOrNil("precipitation_sum_mm", precipitationSumMM); err != nil {
		return WeatherDay{}, err
	}
	return WeatherDay{
		date:               date,
		temperatureMeanC:   copyFloat(temperatureMeanC),
		precipitationSumMM: copyFloat(precipitationSumMM),
	}, nil
}

func (d WeatherDay) Date() Date {
	return d.date
}

func (d WeatherDay) TemperatureMeanC() *float64 {
	return copyFloat(d.temperatureMeanC)
}

func (d WeatherDay) PrecipitationSumMM() *float64 {
	return copyFloat(d.precipitationSumMM)
}

type WeatherSeries struct {
	descriptor Descriptor
	cell       geom.Coordinate
	days       []WeatherDay
	notes      []string
}

func NewWeatherSeries(descriptor Descriptor, cell geom.Coordinate, days []WeatherDay, notes []string) (WeatherSeries, error) {
	if descriptor.Kind() != KindWeather {
		return WeatherSeries{}, fmt.Errorf("weather source has kind %q", descriptor.Kind())
	}
	stored := make([]WeatherDay, len(days))
	copy(stored, days)
	storedNotes := make([]string, len(notes))
	copy(storedNotes, notes)
	return WeatherSeries{descriptor: descriptor, cell: cell, days: stored, notes: storedNotes}, nil
}

func (s WeatherSeries) Descriptor() Descriptor {
	return s.descriptor
}

func (s WeatherSeries) Cell() geom.Coordinate {
	return s.cell
}

func (s WeatherSeries) Days() []WeatherDay {
	days := make([]WeatherDay, len(s.days))
	copy(days, s.days)
	return days
}

func (s WeatherSeries) Notes() []string {
	notes := make([]string, len(s.notes))
	copy(notes, s.notes)
	return notes
}

type WeatherProvider interface {
	FetchDaily(ctx context.Context, request WeatherRequest) (WeatherSeries, error)
}

func intervalMidpoint(interval DateRange) Date {
	return interval.From().AddDays((interval.Days() - 1) / 2)
}
