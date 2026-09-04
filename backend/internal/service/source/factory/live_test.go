//go:build live

package factory_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/integration/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/overpass"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
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
		t.Fatalf("settings were not built: %v", err)
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
		t.Fatalf("contour finder was not built: %v", err)
	}
	bbox, err := geom.ParseBBox(liveBBox)
	if err != nil {
		t.Fatalf("search bbox was not built: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := finder.FindContours(ctx, bbox)
	if err != nil {
		t.Fatalf("contours were not fetched: %v", err)
	}
	t.Logf("contours: %d, версия данных OSM: %s", result.Count(), result.Origin().UpstreamVersion())
	if result.IsEmpty() {
		t.Skip("no landuse=farmland contours in selected area")
	}
	if result.Contours()[0].Polygon().AreaHectares() <= 0 {
		t.Fatal("found contour area was not calculated")
	}
}

func TestLiveOpenMeteoReturnsWeather(t *testing.T) {
	settings := liveSettings(t)
	config := openmeteo.DefaultConfig()
	config.PrimaryEndpoint = settings.WeatherURL
	config.FallbackEndpoint = settings.WeatherFallbackURL
	provider, err := openmeteo.NewProvider(config)
	if err != nil {
		t.Fatalf("weather provider was not built: %v", err)
	}
	period, err := source.ParseDateRange(livePeriodFrom, livePeriodTo)
	if err != nil {
		t.Fatalf("period was not built: %v", err)
	}
	request, err := source.NewWeatherRequest(geom.MustCoordinate(39.0, 45.25), period)
	if err != nil {
		t.Fatalf("weather request was not built: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	series, err := provider.FetchDaily(ctx, request)
	if err != nil {
		t.Fatalf("weather was not fetched: %v", err)
	}
	if len(series.Days()) == 0 {
		t.Fatal("weather series is empty")
	}
	t.Logf("days: %d, ячейка: %v", len(series.Days()), series.Cell())
}

func TestLiveCollectorProcessesPolygon(t *testing.T) {
	settings := liveSettings(t)
	if settings.CDSEClientID == "" || settings.CDSEClientSecret == "" {
		t.Skipf("CDSE access is not configured: set %s и %s", factory.EnvCDSEClientID, factory.EnvCDSEClientSecret)
	}
	assembly, err := factory.New(settings)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
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
		t.Fatalf("polygon was not built: %v", err)
	}
	period, err := source.ParseDateRange(livePeriodFrom, livePeriodTo)
	if err != nil {
		t.Fatalf("period was not built: %v", err)
	}
	request, err := source.NewCollectRequest("area-live-1", polygon, period)
	if err != nil {
		t.Fatalf("collection request was not built: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	snapshot, err := assembly.Collector().Collect(ctx, request)
	if err != nil {
		t.Fatalf("collection failed: %v", err)
	}
	analyze, err := source.NewAnalyzeRequestBuilder(0, 0).Build(snapshot, "job-live-1")
	if err != nil {
		t.Fatalf("analysis request was not built: %v", err)
	}
	t.Logf("input revision: %s, наблюдений: %d, пригодных: %d, ограничения: %v",
		snapshot.Revision(), analyze.ObservationCount(), snapshot.UsableCount(), snapshot.Limitations())
}
