package overpass_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/overpass"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("..", "testdata", "overpass", name))
	if err != nil {
		t.Fatalf("фикстура %s не прочитана: %v", name, err)
	}
	return payload
}

func serve(t *testing.T, body []byte, capturedQuery *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if capturedQuery != nil {
			if err := request.ParseForm(); err == nil {
				*capturedQuery = request.PostForm.Get("data")
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func newFinder(t *testing.T, endpoint string, limits source.Limits, maxResults int) *overpass.Finder {
	t.Helper()
	config := overpass.DefaultConfig()
	config.PrimaryEndpoint = endpoint
	config.FallbackEndpoint = ""
	config.Limits = limits
	config.MaxResults = maxResults
	config.Clock = func() time.Time { return time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC) }
	finder, err := overpass.NewFinder(config)
	if err != nil {
		t.Fatalf("поиск контуров не построен: %v", err)
	}
	return finder
}

func searchArea(t *testing.T) geom.BBox {
	t.Helper()
	bbox, err := geom.ParseBBox("38.90,45.20,39.10,45.35")
	if err != nil {
		t.Fatalf("область поиска не построена: %v", err)
	}
	return bbox
}

func TestFinderConvertsRealResponse(t *testing.T) {
	query := ""
	server := serve(t, fixture(t, "farmland_krasnodar.json"), &query)

	result, err := newFinder(t, server.URL, source.DefaultLimits(), 200).FindContours(context.Background(), searchArea(t))
	if err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	if result.Count() != 3 {
		t.Fatalf("контуров %d, ожидалось 3", result.Count())
	}
	contour := result.Contours()[0]
	if !strings.HasPrefix(contour.ID(), "osm/way/") {
		t.Fatalf("идентификатор контура %q", contour.ID())
	}
	if contour.Polygon().AreaHectares() <= 0 {
		t.Fatal("площадь контура не вычислена")
	}
	if contour.Tags()["landuse"] != "farmland" {
		t.Fatalf("теги контура %v", contour.Tags())
	}
	origin := result.Origin()
	if origin.License() != overpass.License || origin.Attribution() != overpass.Attribution {
		t.Fatalf("лицензия %q, атрибуция %q", origin.License(), origin.Attribution())
	}
	if origin.UpstreamVersion() == "" {
		t.Fatal("версия данных OSM не сохранена")
	}
	if !strings.Contains(origin.Query(), "landuse") || origin.Query() != query {
		t.Fatalf("запрос источника не сохранён: %q против %q", origin.Query(), query)
	}
	if !strings.Contains(strings.Join(result.Notes(), " "), "не юридическая граница") {
		t.Fatalf("предупреждение о смысле landuse отсутствует: %v", result.Notes())
	}
	if result.Truncated() {
		t.Fatal("результат помечен усечённым без достижения предела")
	}
}

func TestFinderTreatsEmptyAnswerAsResultWithoutError(t *testing.T) {
	server := serve(t, fixture(t, "empty_area.json"), nil)

	result, err := newFinder(t, server.URL, source.DefaultLimits(), 200).FindContours(context.Background(), searchArea(t))
	if err != nil {
		t.Fatalf("пустая выдача не должна быть ошибкой: %v", err)
	}
	if !result.IsEmpty() {
		t.Fatalf("контуров %d, ожидалась пустая выдача", result.Count())
	}
	if !strings.Contains(strings.Join(result.Notes(), " "), "не найдены") {
		t.Fatalf("пустая выдача не описана: %v", result.Notes())
	}
	if result.Origin().RetrievedAt().IsZero() {
		t.Fatal("время получения пустой выдачи не сохранено")
	}
}

func TestFinderMarksTruncatedResult(t *testing.T) {
	server := serve(t, fixture(t, "farmland_krasnodar.json"), nil)

	result, err := newFinder(t, server.URL, source.DefaultLimits(), 3).FindContours(context.Background(), searchArea(t))
	if err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	if !result.Truncated() {
		t.Fatal("достижение предела выдачи не отмечено")
	}
	if !strings.Contains(strings.Join(result.Notes(), " "), "уменьшите область поиска") {
		t.Fatalf("подсказка об усечении отсутствует: %v", result.Notes())
	}
}

func TestFinderFiltersContoursOutsideAreaLimits(t *testing.T) {
	limits, err := source.NewLimits(source.LimitsSpec{
		MinAreaHectares:     0.5,
		MaxAreaHectares:     1,
		MaxPolygonVertices:  512,
		MaxPeriodDays:       732,
		MaxObservations:     4096,
		MaxSearchAreaSquare: 250000,
	})
	if err != nil {
		t.Fatalf("пределы не построены: %v", err)
	}
	server := serve(t, fixture(t, "farmland_krasnodar.json"), nil)

	result, err := newFinder(t, server.URL, limits, 200).FindContours(context.Background(), searchArea(t))
	if err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	if !result.IsEmpty() {
		t.Fatalf("контуров %d, ожидалась фильтрация по площади", result.Count())
	}
	if !strings.Contains(strings.Join(result.Notes(), " "), "вне пределов площади") {
		t.Fatalf("фильтрация по площади не описана: %v", result.Notes())
	}
}

func TestFinderRejectsTooLargeSearchArea(t *testing.T) {
	server := serve(t, fixture(t, "empty_area.json"), nil)
	limits, err := source.NewLimits(source.LimitsSpec{
		MinAreaHectares:     0.5,
		MaxAreaHectares:     25000,
		MaxPolygonVertices:  512,
		MaxPeriodDays:       732,
		MaxObservations:     4096,
		MaxSearchAreaSquare: 10,
	})
	if err != nil {
		t.Fatalf("пределы не построены: %v", err)
	}
	_, err = newFinder(t, server.URL, limits, 200).FindContours(context.Background(), searchArea(t))
	if source.KindOf(err) != source.FailureLimitExceeded {
		t.Fatalf("вид ошибки %q", source.KindOf(err))
	}
}

func TestFinderSkipsBrokenGeometry(t *testing.T) {
	body := `{"version":0.6,"generator":"test","osm3s":{"timestamp_osm_base":"2026-09-04T19:02:51Z"},"elements":[
	  {"type":"way","id":1,"geometry":[{"lat":45.0,"lon":39.0},{"lat":45.0,"lon":39.01}],"tags":{"landuse":"farmland"}},
	  {"type":"node","id":2,"lat":45.0,"lon":39.0},
	  {"type":"way","id":3,"geometry":[{"lat":45.0,"lon":39.0},{"lat":45.0,"lon":39.01},{"lat":45.01,"lon":39.01},{"lat":45.01,"lon":39.0},{"lat":45.0,"lon":39.0}],"tags":{"landuse":"farmland","name":"Поле 3"}}
	]}`
	server := serve(t, []byte(body), nil)

	result, err := newFinder(t, server.URL, source.DefaultLimits(), 200).FindContours(context.Background(), searchArea(t))
	if err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	if result.Count() != 1 {
		t.Fatalf("контуров %d, ожидался 1 корректный", result.Count())
	}
	if result.Contours()[0].Name() != "Поле 3" {
		t.Fatalf("название контура %q", result.Contours()[0].Name())
	}
	if !strings.Contains(strings.Join(result.Notes(), " "), "некорректной геометрией") {
		t.Fatalf("пропуск некорректной геометрии не описан: %v", result.Notes())
	}
}

func TestFinderMapsProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := newFinder(t, server.URL, source.DefaultLimits(), 200).FindContours(context.Background(), searchArea(t))
	if source.KindOf(err) != source.FailureRateLimited {
		t.Fatalf("вид ошибки %q", source.KindOf(err))
	}
}

func TestFinderQueryUsesOverpassBoundingBoxOrder(t *testing.T) {
	query := ""
	server := serve(t, fixture(t, "empty_area.json"), &query)

	if _, err := newFinder(t, server.URL, source.DefaultLimits(), 200).FindContours(context.Background(), searchArea(t)); err != nil {
		t.Fatalf("контуры не получены: %v", err)
	}
	if !strings.Contains(query, "(45.2000000,38.9000000,45.3500000,39.1000000)") {
		t.Fatalf("порядок координат в запросе неверен: %s", query)
	}
}
