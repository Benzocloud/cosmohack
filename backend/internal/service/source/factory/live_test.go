//go:build live

package factory_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/overpass"
)

const (
	liveBBox       = "38.90,45.20,39.10,45.35"
	livePeriodFrom = "2025-06-01"
	livePeriodTo   = "2025-06-30"
)

func liveSettings(t *testing.T) factory.Settings {
	t.Helper()
	settings, err := factory.SettingsFromEnv(os.LookupEnv)
	if err != nil {
		t.Fatalf("настройки не собраны: %v", err)
	}
	return settings
}

func TestLiveOverpassReturnsContours(t *testing.T) {
	settings := liveSettings(t)
	config := overpass.DefaultConfig()
	config.PrimaryEndpoint = settings.OverpassURL
	config.FallbackEndpoint = settings.OverpassFallbackURL
	finder, err := overpass.NewFinder(config)
	if err != nil {
		t.Fatalf("поиск контуров не построен: %v", err)
	}
	bbox, err := geom.ParseBBox(liveBBox)
	if err != nil {
		t.Fatalf("область поиска не построена: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := finder.FindContours(ctx, bbox)
	if err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	t.Logf("контуров: %d, версия данных OSM: %s", result.Count(), result.Origin().UpstreamVersion())
	if result.IsEmpty() {
		t.Skip("в выбранной области нет контуров landuse=farmland")
	}
	if result.Contours()[0].Polygon().AreaHectares() <= 0 {
		t.Fatal("площадь найденного контура не вычислена")
	}
}

func TestLiveOpenMeteoReturnsWeather(t *testing.T) {
	settings := liveSettings(t)
	config := openmeteo.DefaultConfig()
	config.PrimaryEndpoint = settings.WeatherURL
	config.FallbackEndpoint = settings.WeatherFallbackURL
	provider, err := openmeteo.NewProvider(config)
	if err != nil {
		t.Fatalf("провайдер погоды не построен: %v", err)
	}
	period, err := source.ParseDateRange(livePeriodFrom, livePeriodTo)
	if err != nil {
		t.Fatalf("период не построен: %v", err)
	}
	request, err := source.NewWeatherRequest(geom.MustCoordinate(39.0, 45.25), period)
	if err != nil {
		t.Fatalf("запрос погоды не построен: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	series, err := provider.FetchDaily(ctx, request)
	if err != nil {
		t.Fatalf("погода не получена: %v", err)
	}
	if len(series.Days()) == 0 {
		t.Fatal("погодный ряд пуст")
	}
	t.Logf("дней: %d, ячейка: %v", len(series.Days()), series.Cell())
}

func TestLiveCollectorProcessesPolygon(t *testing.T) {
	settings := liveSettings(t)
	if settings.CDSEClientID == "" || settings.CDSEClientSecret == "" {
		t.Skipf("доступ к CDSE не настроен: задайте %s и %s", factory.EnvCDSEClientID, factory.EnvCDSEClientSecret)
	}
	assembly, err := factory.New(settings)
	if err != nil {
		t.Fatalf("сборка не выполнена: %v", err)
	}
	ring := []geom.Coordinate{
		geom.MustCoordinate(38.9746397, 45.2056541),
		geom.MustCoordinate(38.9814204, 45.2056541),
		geom.MustCoordinate(38.9814204, 45.2100000),
		geom.MustCoordinate(38.9746397, 45.2100000),
		geom.MustCoordinate(38.9746397, 45.2056541),
	}
	polygon, err := geom.NewPolygon(ring)
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	period, err := source.ParseDateRange(livePeriodFrom, livePeriodTo)
	if err != nil {
		t.Fatalf("период не построен: %v", err)
	}
	request, err := source.NewCollectRequest("area-live-1", polygon, period)
	if err != nil {
		t.Fatalf("запрос сбора не построен: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	snapshot, err := assembly.Collector().Collect(ctx, request)
	if err != nil {
		t.Fatalf("сбор не выполнен: %v", err)
	}
	analyze, err := source.NewAnalyzeRequestBuilder(0, 0).Build(snapshot, "job-live-1")
	if err != nil {
		t.Fatalf("запрос анализа не построен: %v", err)
	}
	t.Logf("версия входа: %s, наблюдений: %d, пригодных: %d, ограничения: %v",
		snapshot.Revision(), analyze.ObservationCount(), snapshot.UsableCount(), snapshot.Limitations())
}
