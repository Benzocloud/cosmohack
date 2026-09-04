package source

import (
	"context"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

type Clock func() time.Time

type CollectRequest struct {
	areaID  string
	polygon *geom.Polygon
	period  DateRange
}

func NewCollectRequest(areaID string, polygon *geom.Polygon, period DateRange) (CollectRequest, error) {
	if err := requireIdentifier("area_id", areaID); err != nil {
		return CollectRequest{}, NewProviderError(FailureInvalidRequest, limitsProvider, "%v", err)
	}
	if polygon == nil {
		return CollectRequest{}, NewProviderError(FailureInvalidRequest, limitsProvider, "геометрия участка не задана")
	}
	if period.IsZero() {
		return CollectRequest{}, NewProviderError(FailureInvalidRequest, limitsProvider, "период анализа не задан")
	}
	return CollectRequest{areaID: areaID, polygon: polygon, period: period}, nil
}

func (r CollectRequest) AreaID() string {
	return r.areaID
}

func (r CollectRequest) Polygon() *geom.Polygon {
	return r.polygon
}

func (r CollectRequest) Period() DateRange {
	return r.period
}

type CollectorOption func(*Collector) error

func WithLimits(limits Limits) CollectorOption {
	return func(collector *Collector) error {
		collector.limits = limits
		return nil
	}
}

func WithClock(clock Clock) CollectorOption {
	return func(collector *Collector) error {
		if clock == nil {
			return fmt.Errorf("часы сборщика не заданы")
		}
		collector.clock = clock
		return nil
	}
}

type Collector struct {
	satellite SatelliteProvider
	weather   WeatherProvider
	limits    Limits
	clock     Clock
}

func NewCollector(satellite SatelliteProvider, weather WeatherProvider, options ...CollectorOption) (*Collector, error) {
	if satellite == nil {
		return nil, fmt.Errorf("сборщик без спутникового источника")
	}
	if weather == nil {
		return nil, fmt.Errorf("сборщик без погодного источника")
	}
	collector := &Collector{
		satellite: satellite,
		weather:   weather,
		limits:    DefaultLimits(),
		clock:     time.Now,
	}
	for _, option := range options {
		if err := option(collector); err != nil {
			return nil, err
		}
	}
	return collector, nil
}

func (c *Collector) Limits() Limits {
	return c.limits
}

func (c *Collector) Collect(ctx context.Context, request CollectRequest) (*Snapshot, error) {
	if err := c.limits.ValidatePolygon(request.polygon); err != nil {
		return nil, err
	}
	if err := c.limits.ValidatePeriod(request.period); err != nil {
		return nil, err
	}
	satelliteRequest, err := NewSatelliteRequest(request.polygon, request.period)
	if err != nil {
		return nil, NewProviderError(FailureInvalidRequest, limitsProvider, "%v", err)
	}
	satellite, err := c.satellite.FetchNDVI(ctx, satelliteRequest)
	if err != nil {
		return nil, err
	}

	limitations := make([]string, 0, 8)
	limitations = append(limitations, satellite.Notes()...)
	samples, sampleNotes := indexSamples(satellite.Samples(), request.period)
	limitations = append(limitations, sampleNotes...)
	if len(samples) == 0 {
		limitations = append(limitations, "Спутниковый источник не вернул наблюдений за выбранный период")
	}

	point := request.polygon.RepresentativePoint()
	weather, weatherCell, weatherNotes := c.collectWeather(ctx, point, request.period)
	limitations = append(limitations, weatherNotes...)

	descriptors := []Descriptor{satellite.Descriptor()}
	weatherSourceID := ""
	if weatherCell != nil {
		descriptors = append(descriptors, weather.Descriptor())
		weatherSourceID = weather.Descriptor().ID()
	}

	observations, err := c.buildObservations(request.period, samples, weather, satellite.Descriptor().ID(), weatherSourceID)
	if err != nil {
		return nil, err
	}
	if usable := countUsable(observations); usable == 0 {
		limitations = append(limitations, "Пригодных спутниковых наблюдений в периоде нет")
	}

	return NewSnapshot(SnapshotSpec{
		AreaID:       request.areaID,
		Period:       request.period,
		Polygon:      request.polygon,
		Descriptors:  descriptors,
		Observations: observations,
		WeatherCell:  weatherCell,
		Limitations:  limitations,
		CollectedAt:  c.clock().UTC(),
	})
}

func (c *Collector) collectWeather(ctx context.Context, point geom.Coordinate, period DateRange) (WeatherSeries, *geom.Coordinate, []string) {
	request, err := NewWeatherRequest(point, period)
	if err != nil {
		return WeatherSeries{}, nil, []string{fmt.Sprintf("Погодный запрос не построен: %v", err)}
	}
	series, err := c.weather.FetchDaily(ctx, request)
	if err != nil {
		return WeatherSeries{}, nil, []string{fmt.Sprintf("Погодные данные не получены (%s); анализ выполняется без погодного контекста", KindOfOrUnknown(err))}
	}
	notes := series.Notes()
	if covered := len(series.Days()); covered < period.Days() {
		notes = append(notes, fmt.Sprintf("Погода получена на %d из %d дней периода", covered, period.Days()))
	}
	cell := series.Cell()
	return series, &cell, notes
}

func (c *Collector) buildObservations(period DateRange, samples map[string]SatelliteSample, weather WeatherSeries, satelliteSourceID, weatherSourceID string) ([]Observation, error) {
	weatherDays := indexWeather(weather.Days())
	observations := make([]Observation, 0, period.Days())
	for _, date := range period.Dates() {
		builder := NewObservationBuilder(date)
		switch sample, found := samples[date.String()]; {
		case found && sample.Usable():
			builder.Measured(*sample.NDVI(), satelliteSourceID, sample.Interval(), sample.ValidFraction())
		case found:
			builder.Rejected(sample.NDVI(), satelliteSourceID, sample.Interval(), sample.Reason(), sample.ValidFraction())
		default:
			builder.Missing(ReasonNoObservation)
		}
		if day, found := weatherDays[date.String()]; found && weatherSourceID != "" {
			point, err := NewWeather(weatherSourceID, day.TemperatureMeanC(), day.PrecipitationSumMM())
			if err != nil {
				return nil, err
			}
			builder.WithWeather(point)
		}
		observation, err := builder.Build()
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func indexSamples(samples []SatelliteSample, period DateRange) (map[string]SatelliteSample, []string) {
	index := make(map[string]SatelliteSample, len(samples))
	notes := make([]string, 0, 2)
	outside, conflicts := 0, 0
	for _, sample := range samples {
		if !period.Contains(sample.Date()) {
			outside++
			continue
		}
		key := sample.Date().String()
		previous, exists := index[key]
		if !exists {
			index[key] = sample
			continue
		}
		conflicts++
		if preferSample(sample, previous) {
			index[key] = sample
		}
	}
	if outside > 0 {
		notes = append(notes, fmt.Sprintf("Наблюдений вне периода анализа: %d", outside))
	}
	if conflicts > 0 {
		notes = append(notes, fmt.Sprintf("Совпадающих по дате интервалов: %d, оставлено наблюдение с лучшим качеством", conflicts))
	}
	return index, notes
}

func preferSample(candidate, current SatelliteSample) bool {
	if candidate.Usable() != current.Usable() {
		return candidate.Usable()
	}
	candidateFraction, currentFraction := candidate.ValidFraction(), current.ValidFraction()
	if candidateFraction == nil {
		return false
	}
	if currentFraction == nil {
		return true
	}
	return *candidateFraction > *currentFraction
}

func indexWeather(days []WeatherDay) map[string]WeatherDay {
	index := make(map[string]WeatherDay, len(days))
	for _, day := range days {
		index[day.Date().String()] = day
	}
	return index
}

func countUsable(observations []Observation) int {
	count := 0
	for _, observation := range observations {
		if observation.Quality() == QualityUsable {
			count++
		}
	}
	return count
}
