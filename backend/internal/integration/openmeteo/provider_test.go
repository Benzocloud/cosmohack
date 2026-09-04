package openmeteo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("фикстура %s не прочитана: %v", name, err)
	}
	return payload
}

func serve(t *testing.T, body []byte, captured *url.Values) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if captured != nil {
			*captured = request.URL.Query()
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func newProvider(t *testing.T, endpoint string) *openmeteo.Provider {
	t.Helper()
	config := openmeteo.DefaultConfig()
	config.PrimaryEndpoint = endpoint
	config.FallbackEndpoint = ""
	config.Clock = func() time.Time { return time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC) }
	provider, err := openmeteo.NewProvider(config)
	if err != nil {
		t.Fatalf("провайдер не построен: %v", err)
	}
	return provider
}

func request(t *testing.T, from, to string) source.WeatherRequest {
	t.Helper()
	period, err := source.ParseDateRange(from, to)
	if err != nil {
		t.Fatalf("период не построен: %v", err)
	}
	weatherRequest, err := source.NewWeatherRequest(geom.MustCoordinate(38.9746397, 45.2056541), period)
	if err != nil {
		t.Fatalf("запрос погоды не построен: %v", err)
	}
	return weatherRequest
}

func TestProviderParsesRealResponse(t *testing.T) {
	query := url.Values{}
	server := serve(t, fixture(t, "era5_krasnodar.json"), &query)

	series, err := newProvider(t, server.URL).FetchDaily(context.Background(), request(t, "2025-06-01", "2025-06-10"))
	if err != nil {
		t.Fatalf("погода не получена: %v", err)
	}
	if len(series.Days()) != 10 {
		t.Fatalf("дней %d, ожидалось 10", len(series.Days()))
	}
	if series.Cell().Lat() != 45.25 || series.Cell().Lon() != 39.0 {
		t.Fatalf("ячейка реанализа %v не соответствует ответу", series.Cell())
	}
	first := series.Days()[0]
	if first.Date().String() != "2025-06-01" || *first.TemperatureMeanC() != 20.6 || *first.PrecipitationSumMM() != 0 {
		t.Fatalf("первый день разобран неверно: %v", first)
	}
	descriptor := series.Descriptor()
	if descriptor.ID() != openmeteo.SourceID || descriptor.Kind() != source.KindWeather {
		t.Fatalf("описание источника %s/%s", descriptor.ID(), descriptor.Kind())
	}
	if descriptor.License() == nil || *descriptor.License() == "" {
		t.Fatal("лицензия источника не заполнена")
	}
	if !strings.Contains(descriptor.Mapping(), "ячейка реанализа") || !strings.Contains(descriptor.Mapping(), "UTC") {
		t.Fatalf("сопоставление источника не описывает ячейку и UTC: %s", descriptor.Mapping())
	}
	if query.Get("models") != "era5" || query.Get("timezone") != "UTC" {
		t.Fatalf("запрос выполнен с моделью %q и зоной %q", query.Get("models"), query.Get("timezone"))
	}
	if query.Get("start_date") != "2025-06-01" || query.Get("end_date") != "2025-06-10" {
		t.Fatalf("границы запроса %q..%q", query.Get("start_date"), query.Get("end_date"))
	}
	if query.Get("daily") != "temperature_2m_mean,precipitation_sum" {
		t.Fatalf("набор переменных %q", query.Get("daily"))
	}
}

func TestProviderRejectsUnexpectedUnitsAndTimezone(t *testing.T) {
	cases := map[string]string{
		"температура в фаренгейтах": `{"latitude":45.25,"longitude":39.0,"elevation":27.0,"utc_offset_seconds":0,"timezone":"GMT","daily_units":{"time":"iso8601","temperature_2m_mean":"°F","precipitation_sum":"mm"},"daily":{"time":["2025-06-01"],"temperature_2m_mean":[69.0],"precipitation_sum":[0.0]}}`,
		"осадки в дюймах":           `{"latitude":45.25,"longitude":39.0,"elevation":27.0,"utc_offset_seconds":0,"timezone":"GMT","daily_units":{"time":"iso8601","temperature_2m_mean":"°C","precipitation_sum":"inch"},"daily":{"time":["2025-06-01"],"temperature_2m_mean":[20.0],"precipitation_sum":[0.0]}}`,
		"сдвиг времени":             `{"latitude":45.25,"longitude":39.0,"elevation":27.0,"utc_offset_seconds":10800,"timezone":"Europe/Moscow","daily_units":{"time":"iso8601","temperature_2m_mean":"°C","precipitation_sum":"mm"},"daily":{"time":["2025-06-01"],"temperature_2m_mean":[20.0],"precipitation_sum":[0.0]}}`,
		"разная длина массивов":     `{"latitude":45.25,"longitude":39.0,"elevation":27.0,"utc_offset_seconds":0,"timezone":"GMT","daily_units":{"time":"iso8601","temperature_2m_mean":"°C","precipitation_sum":"mm"},"daily":{"time":["2025-06-01","2025-06-02"],"temperature_2m_mean":[20.0],"precipitation_sum":[0.0]}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := serve(t, []byte(body), nil)
			_, err := newProvider(t, server.URL).FetchDaily(context.Background(), request(t, "2025-06-01", "2025-06-02"))
			if domain.KindOf(err) != domain.FailureMalformed {
				t.Fatalf("вид ошибки %q, ожидался %q", domain.KindOf(err), domain.FailureMalformed)
			}
		})
	}
}

func TestProviderKeepsMissingValuesAsNull(t *testing.T) {
	body := `{"latitude":45.25,"longitude":39.0,"elevation":27.0,"utc_offset_seconds":0,"timezone":"GMT","daily_units":{"time":"iso8601","temperature_2m_mean":"°C","precipitation_sum":"mm"},"daily":{"time":["2025-06-01","2025-06-02"],"temperature_2m_mean":[null,21.0],"precipitation_sum":[1.5,-3.0]}}`
	server := serve(t, []byte(body), nil)

	series, err := newProvider(t, server.URL).FetchDaily(context.Background(), request(t, "2025-06-01", "2025-06-02"))
	if err != nil {
		t.Fatalf("погода не получена: %v", err)
	}
	if series.Days()[0].TemperatureMeanC() != nil {
		t.Fatal("отсутствующая температура заменена значением")
	}
	if series.Days()[1].PrecipitationSumMM() != nil {
		t.Fatal("отрицательные осадки приняты как значение")
	}
	if len(series.Notes()) == 0 {
		t.Fatal("замена отрицательных осадков не отмечена")
	}
}

func TestProviderDropsDaysOutsidePeriod(t *testing.T) {
	server := serve(t, fixture(t, "era5_krasnodar.json"), nil)

	series, err := newProvider(t, server.URL).FetchDaily(context.Background(), request(t, "2025-06-01", "2025-06-03"))
	if err != nil {
		t.Fatalf("погода не получена: %v", err)
	}
	if len(series.Days()) != 3 {
		t.Fatalf("дней %d, ожидалось 3", len(series.Days()))
	}
	if len(series.Notes()) == 0 {
		t.Fatal("дни вне периода не отмечены")
	}
}

func TestProviderReportsProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := newProvider(t, server.URL).FetchDaily(context.Background(), request(t, "2025-06-01", "2025-06-02"))
	if domain.KindOf(err) != domain.FailureRateLimited {
		t.Fatalf("вид ошибки %q", domain.KindOf(err))
	}
}
