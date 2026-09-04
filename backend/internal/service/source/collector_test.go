package source_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

type stubSatellite struct {
	series  source.SatelliteSeries
	failure error
	calls   int
}

func (s *stubSatellite) FetchNDVI(context.Context, source.SatelliteRequest) (source.SatelliteSeries, error) {
	s.calls++
	if s.failure != nil {
		return source.SatelliteSeries{}, s.failure
	}
	return s.series, nil
}

type stubWeather struct {
	series  source.WeatherSeries
	failure error
	request source.WeatherRequest
}

func (s *stubWeather) FetchDaily(_ context.Context, request source.WeatherRequest) (source.WeatherSeries, error) {
	s.request = request
	if s.failure != nil {
		return source.WeatherSeries{}, s.failure
	}
	return s.series, nil
}

func fixedClock() source.Clock {
	moment := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return moment }
}

func testPolygon(t *testing.T) *geom.Polygon {
	t.Helper()
	ring := []geom.Coordinate{
		geom.MustCoordinate(39.0, 45.0),
		geom.MustCoordinate(39.01, 45.0),
		geom.MustCoordinate(39.01, 45.01),
		geom.MustCoordinate(39.0, 45.01),
		geom.MustCoordinate(39.0, 45.0),
	}
	polygon, err := geom.NewPolygon(ring)
	if err != nil {
		t.Fatalf("тестовый полигон не построен: %v", err)
	}
	return polygon
}

func satelliteDescriptor(t *testing.T) source.Descriptor {
	t.Helper()
	descriptor, err := source.NewDescriptor(source.DescriptorSpec{
		ID:          "satellite-test",
		Kind:        source.KindSatellite,
		Provider:    "test-provider",
		Dataset:     "test-dataset",
		Mapping:     "NDVI среднее по полигону",
		License:     source.License("test-license"),
		RetrievedAt: fixedClock()(),
	})
	if err != nil {
		t.Fatalf("описание спутникового источника не построено: %v", err)
	}
	return descriptor
}

func weatherDescriptor(t *testing.T) source.Descriptor {
	t.Helper()
	descriptor, err := source.NewDescriptor(source.DescriptorSpec{
		ID:          "weather-test",
		Kind:        source.KindWeather,
		Provider:    "test-provider",
		Dataset:     "test-reanalysis",
		Mapping:     "суточная агрегация UTC",
		License:     source.License("test-license"),
		RetrievedAt: fixedClock()(),
	})
	if err != nil {
		t.Fatalf("описание погодного источника не построено: %v", err)
	}
	return descriptor
}

func sampleFor(t *testing.T, from, to string, ndvi, fraction *float64, usable bool, reason string) source.SatelliteSample {
	t.Helper()
	sample, err := source.NewSatelliteSample(mustRange(t, from, to), ndvi, fraction, usable, reason)
	if err != nil {
		t.Fatalf("наблюдение %s..%s не построено: %v", from, to, err)
	}
	return sample
}

func satelliteSeries(t *testing.T, samples ...source.SatelliteSample) source.SatelliteSeries {
	t.Helper()
	series, err := source.NewSatelliteSeries(satelliteDescriptor(t), samples, nil)
	if err != nil {
		t.Fatalf("спутниковый ряд не построен: %v", err)
	}
	return series
}

func weatherSeries(t *testing.T, period source.DateRange) source.WeatherSeries {
	t.Helper()
	days := make([]source.WeatherDay, 0, period.Days())
	for index, date := range period.Dates() {
		day, err := source.NewWeatherDay(date, source.Float(20+float64(index)/10), source.Float(float64(index%3)))
		if err != nil {
			t.Fatalf("погодный день не построен: %v", err)
		}
		days = append(days, day)
	}
	series, err := source.NewWeatherSeries(weatherDescriptor(t), geom.MustCoordinate(39.0, 45.0), days, nil)
	if err != nil {
		t.Fatalf("погодный ряд не построен: %v", err)
	}
	return series
}

func newCollector(t *testing.T, satellite source.SatelliteProvider, weather source.WeatherProvider) *source.Collector {
	t.Helper()
	collector, err := source.NewCollector(satellite, weather, source.WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("сборщик не построен: %v", err)
	}
	return collector
}

func collectRequest(t *testing.T, period source.DateRange) source.CollectRequest {
	t.Helper()
	request, err := source.NewCollectRequest("area-test", testPolygon(t), period)
	if err != nil {
		t.Fatalf("запрос сбора не построен: %v", err)
	}
	return request
}

func TestCollectorBuildsDenseSeries(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-10")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-06-01", "2025-06-05", source.Float(0.71), source.Float(0.95), true, ""),
		sampleFor(t, "2025-06-06", "2025-06-10", source.Float(0.44), source.Float(0.2), false, source.ReasonLowValidFraction),
	)}
	weather := &stubWeather{series: weatherSeries(t, period)}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	observations := snapshot.Observations()
	if len(observations) != 10 {
		t.Fatalf("наблюдений %d, ожидалось 10", len(observations))
	}
	if snapshot.UsableCount() != 1 {
		t.Fatalf("пригодных наблюдений %d, ожидалось 1", snapshot.UsableCount())
	}
	if observations[2].Quality() != source.QualityUsable {
		t.Fatalf("наблюдение 2025-06-03 имеет качество %s", observations[2].Quality())
	}
	if observations[7].Quality() != source.QualityUnusable {
		t.Fatalf("наблюдение 2025-06-08 имеет качество %s", observations[7].Quality())
	}
	if observations[0].Quality() != source.QualityMissing || observations[0].MissingReason() != source.ReasonNoObservation {
		t.Fatalf("день без наблюдения помечен как %s (%s)", observations[0].Quality(), observations[0].MissingReason())
	}
	for _, observation := range observations {
		if observation.Weather() == nil {
			t.Fatalf("погода отсутствует на дате %s", observation.Date())
		}
	}
	if len(snapshot.Descriptors()) != 2 {
		t.Fatalf("источников %d, ожидалось 2", len(snapshot.Descriptors()))
	}
	if snapshot.WeatherCell() == nil {
		t.Fatal("ячейка реанализа не сохранена")
	}
}

func TestCollectorUsesRepresentativePointForWeather(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-03")
	satellite := &stubSatellite{series: satelliteSeries(t)}
	weather := &stubWeather{series: weatherSeries(t, period)}

	if _, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period)); err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	point := weather.request.Point()
	if !testPolygon(t).Contains(point) {
		t.Fatalf("точка запроса погоды %v вне полигона", point)
	}
}

func TestCollectorSurvivesWeatherFailure(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-03")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-06-01", "2025-06-03", source.Float(0.6), source.Float(0.9), true, ""),
	)}
	weather := &stubWeather{failure: domain.NewProviderError(domain.FailureUnavailable, "weather", "сервис недоступен")}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("отказ погоды не должен прерывать сбор: %v", err)
	}
	if len(snapshot.Descriptors()) != 1 {
		t.Fatalf("источников %d, ожидался только спутниковый", len(snapshot.Descriptors()))
	}
	if snapshot.WeatherCell() != nil {
		t.Fatal("ячейка реанализа сохранена без погодных данных")
	}
	for _, observation := range snapshot.Observations() {
		if observation.Weather() != nil {
			t.Fatalf("погода добавлена на дате %s несмотря на отказ источника", observation.Date())
		}
	}
	if !containsSubstring(snapshot.Limitations(), "Погодные данные не получены") {
		t.Fatalf("ограничение об отказе погоды отсутствует: %v", snapshot.Limitations())
	}
}

func TestCollectorReportsPartialWeatherCoverage(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-05")
	satellite := &stubSatellite{series: satelliteSeries(t)}
	weather := &stubWeather{series: weatherSeries(t, mustRange(t, "2025-06-01", "2025-06-03"))}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	if !containsSubstring(snapshot.Limitations(), "Погода получена на 3 из 5") {
		t.Fatalf("ограничение о неполном покрытии погоды отсутствует: %v", snapshot.Limitations())
	}
	observations := snapshot.Observations()
	if observations[4].Weather() != nil {
		t.Fatal("на день без погоды добавлен погодный объект")
	}
}

func TestCollectorFailsWhenSatelliteUnavailable(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-03")
	satellite := &stubSatellite{failure: domain.NewProviderError(domain.FailureUnavailable, "satellite", "сервис недоступен")}
	weather := &stubWeather{series: weatherSeries(t, period)}

	_, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if domain.KindOf(err) != domain.FailureUnavailable {
		t.Fatalf("вид ошибки %q, ожидался %q", domain.KindOf(err), domain.FailureUnavailable)
	}
	if !domain.IsRetryable(err) {
		t.Fatal("отказ доступа должен допускать явный повтор")
	}
}

func TestCollectorMarksEmptySatelliteResponse(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-03")
	satellite := &stubSatellite{series: satelliteSeries(t)}
	weather := &stubWeather{series: weatherSeries(t, period)}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("пустой ответ источника не должен быть ошибкой: %v", err)
	}
	if snapshot.UsableCount() != 0 {
		t.Fatalf("пригодных наблюдений %d", snapshot.UsableCount())
	}
	if !containsSubstring(snapshot.Limitations(), "не вернул наблюдений") {
		t.Fatalf("ограничение о пустом ответе отсутствует: %v", snapshot.Limitations())
	}
	if !containsSubstring(snapshot.Limitations(), "Пригодных спутниковых наблюдений") {
		t.Fatalf("ограничение об отсутствии пригодных наблюдений отсутствует: %v", snapshot.Limitations())
	}
}

func TestCollectorPrefersBetterSampleOnSameDate(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-05")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-06-01", "2025-06-05", source.Float(0.30), source.Float(0.6), true, ""),
		sampleFor(t, "2025-06-01", "2025-06-05", source.Float(0.80), source.Float(0.9), true, ""),
	)}
	weather := &stubWeather{series: weatherSeries(t, period)}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	value := snapshot.Observations()[2].PrimaryNDVI()
	if value == nil || *value != 0.80 {
		t.Fatalf("оставлено наблюдение %v вместо более качественного", value)
	}
	if !containsSubstring(snapshot.Limitations(), "Совпадающих по дате интервалов") {
		t.Fatalf("ограничение о конфликте дат отсутствует: %v", snapshot.Limitations())
	}
}

func TestCollectorIgnoresSamplesOutsidePeriod(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-05")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-05-20", "2025-05-24", source.Float(0.5), source.Float(0.9), true, ""),
	)}
	weather := &stubWeather{series: weatherSeries(t, period)}

	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	if snapshot.UsableCount() != 0 {
		t.Fatalf("наблюдение вне периода попало в ряд: %d", snapshot.UsableCount())
	}
	if !containsSubstring(snapshot.Limitations(), "вне периода анализа") {
		t.Fatalf("ограничение о наблюдениях вне периода отсутствует: %v", snapshot.Limitations())
	}
}

func TestCollectorEnforcesLimits(t *testing.T) {
	limits, err := domainsource.NewLimits(domainsource.LimitsSpec{
		MinAreaHectares:     1,
		MaxAreaHectares:     10,
		MaxPolygonVertices:  512,
		MaxPeriodDays:       3,
		MaxObservations:     4096,
		MaxSearchAreaSquare: 1000,
	})
	if err != nil {
		t.Fatalf("пределы не построены: %v", err)
	}
	satellite := &stubSatellite{series: satelliteSeries(t)}
	weather := &stubWeather{}
	collector, err := source.NewCollector(satellite, weather, source.WithLimits(limits), source.WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("сборщик не построен: %v", err)
	}
	_, err = collector.Collect(context.Background(), collectRequest(t, mustRange(t, "2025-06-01", "2025-06-02")))
	if domain.KindOf(err) != domain.FailureLimitExceeded {
		t.Fatalf("превышение площади не отклонено: %v", err)
	}
	if satellite.calls != 0 {
		t.Fatal("источник вызван до проверки пределов")
	}
}

func TestSnapshotRevisionIsDeterministic(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-05")
	build := func(ndvi float64) *source.Snapshot {
		satellite := &stubSatellite{series: satelliteSeries(t,
			sampleFor(t, "2025-06-01", "2025-06-05", source.Float(ndvi), source.Float(0.9), true, ""),
		)}
		weather := &stubWeather{series: weatherSeries(t, period)}
		snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
		if err != nil {
			t.Fatalf("сбор не выполнен: %v", err)
		}
		return snapshot
	}
	first, second, changed := build(0.7), build(0.7), build(0.8)
	if first.Revision() != second.Revision() {
		t.Fatalf("одинаковый вход дал разные версии: %s и %s", first.Revision(), second.Revision())
	}
	if first.Revision() == changed.Revision() {
		t.Fatal("изменение наблюдения не изменило версию входа")
	}
	if !strings.HasPrefix(first.Revision(), "snap-") {
		t.Fatalf("версия входа %s не соответствует формату", first.Revision())
	}
}

func TestSnapshotJSONKeepsProvenance(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-03")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-06-01", "2025-06-03", source.Float(0.7), source.Float(0.9), true, ""),
	)}
	weather := &stubWeather{series: weatherSeries(t, period)}
	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("снимок не сериализован: %v", err)
	}
	document := map[string]any{}
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("снимок не разобран: %v", err)
	}
	for _, field := range []string{"area_id", "input_revision", "collected_at", "period", "geometry", "area_hectares", "sources", "observations", "limitations"} {
		if _, found := document[field]; !found {
			t.Fatalf("в снимке нет поля %s", field)
		}
	}
	if strings.Contains(string(payload), "NaN") {
		t.Fatal("снимок содержит NaN")
	}
}

func containsSubstring(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}
