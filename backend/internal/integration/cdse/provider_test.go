package cdse_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/cdse"
)

type stubService struct {
	server            *httptest.Server
	tokenCalls        int32
	statisticCall     int32
	lastBody          []byte
	lastAuth          string
	tokenStatus       int
	statusCode        int
	statisticsFixture string
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("фикстура %s не прочитана: %v", name, err)
	}
	return payload
}

func newService(t *testing.T) *stubService {
	t.Helper()
	service := &stubService{tokenStatus: http.StatusOK, statusCode: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&service.tokenCalls, 1)
		if service.tokenStatus != http.StatusOK {
			writer.WriteHeader(service.tokenStatus)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(fixture(t, "token_synthetic.json"))
	})
	mux.HandleFunc("/statistics", func(writer http.ResponseWriter, request *http.Request) {
		atomic.AddInt32(&service.statisticCall, 1)
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("тело запроса не прочитано: %v", err)
		}
		service.lastBody = body
		service.lastAuth = request.Header.Get("Authorization")
		if service.statusCode != http.StatusOK {
			writer.WriteHeader(service.statusCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		name := service.statisticsFixture
		if name == "" {
			name = "statistics_synthetic.json"
		}
		writer.Write(fixture(t, name))
	})
	service.server = httptest.NewServer(mux)
	t.Cleanup(service.server.Close)
	return service
}

func TestProviderAcceptsNaNStatisticsAsNoValidSamples(t *testing.T) {
	service := newService(t)
	service.statisticsFixture = "statistics_nan.json"

	series, err := newProvider(t, service, fixedClock()).FetchNDVI(
		context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-05"),
	)
	if err != nil {
		t.Fatalf("статистика с NaN должна быть обработана: %v", err)
	}
	if samples := series.Samples(); len(samples) != 1 || samples[0].Usable() ||
		samples[0].Reason() != source.ReasonNoValidSamples {
		t.Fatalf("интервал с NaN разобран неверно: %v", series.Samples())
	}
}

func newProvider(t *testing.T, service *stubService, clock domain.Clock) *cdse.Provider {
	t.Helper()
	credentials, err := cdse.NewCredentials("client", "secret")
	if err != nil {
		t.Fatalf("доступ не построен: %v", err)
	}
	config := cdse.DefaultConfig(credentials)
	config.StatisticsEndpoint = service.server.URL + "/statistics"
	config.TokenEndpoint = service.server.URL + "/token"
	config.Clock = clock
	provider, err := cdse.NewProvider(config)
	if err != nil {
		t.Fatalf("провайдер не построен: %v", err)
	}
	return provider
}

func fixedClock() domain.Clock {
	moment := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return moment }
}

func satelliteRequest(t *testing.T, from, to string) source.SatelliteRequest {
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
		t.Fatalf("полигон не построен: %v", err)
	}
	period, err := source.ParseDateRange(from, to)
	if err != nil {
		t.Fatalf("период не построен: %v", err)
	}
	request, err := source.NewSatelliteRequest(polygon, period)
	if err != nil {
		t.Fatalf("запрос не построен: %v", err)
	}
	return request
}

func TestProviderMapsStatisticsToSamples(t *testing.T) {
	service := newService(t)
	series, err := newProvider(t, service, fixedClock()).FetchNDVI(context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-20"))
	if err != nil {
		t.Fatalf("наблюдения не получены: %v", err)
	}
	samples := series.Samples()
	if len(samples) != 4 {
		t.Fatalf("наблюдений %d, ожидалось 4", len(samples))
	}

	usable := samples[0]
	if !usable.Usable() || *usable.NDVI() != 0.71 {
		t.Fatalf("первое наблюдение %v", usable)
	}
	if usable.Interval().To().String() != "2025-06-05" {
		t.Fatalf("конец интервала %s, ожидался последний включённый день", usable.Interval().To())
	}
	if usable.Date().String() != "2025-06-03" {
		t.Fatalf("дата наблюдения %s, ожидалась середина интервала", usable.Date())
	}
	if *usable.ValidFraction() != 0.95 {
		t.Fatalf("доля пригодной площади %g", *usable.ValidFraction())
	}

	low := samples[1]
	if low.Usable() || low.Reason() != source.ReasonLowValidFraction || *low.NDVI() != 0.44 {
		t.Fatalf("наблюдение с низким качеством разобрано неверно: %v", low)
	}

	empty := samples[2]
	if empty.Usable() || empty.NDVI() != nil || empty.Reason() != source.ReasonNoValidSamples {
		t.Fatalf("наблюдение без пригодных пикселей разобрано неверно: %v", empty)
	}

	failed := samples[3]
	if failed.Usable() || failed.Reason() != source.ReasonProviderEntryFailed {
		t.Fatalf("интервал с ошибкой провайдера разобран неверно: %v", failed)
	}
	if len(series.Notes()) == 0 {
		t.Fatal("ошибка расчёта интервала не отмечена")
	}
}

func TestProviderFetchesS2Indices(t *testing.T) {
	service := newService(t)
	service.statisticsFixture = "statistics_indices_synthetic.json"

	series, err := newProvider(t, service, fixedClock()).FetchIndices(
		context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-15"),
	)
	if err != nil {
		t.Fatalf("индексы не получены: %v", err)
	}
	samples := series.Samples()
	if len(samples) != 3 {
		t.Fatalf("наблюдений %d, ожидалось 3", len(samples))
	}
	first := samples[0]
	indices := first.Indices()
	validIndices := indices.NDVI != nil && *indices.NDVI == 0.71 &&
		indices.EVI != nil && *indices.EVI == 0.62 &&
		indices.NDWI != nil && *indices.NDWI == 0.18
	if !first.Usable() || !validIndices {
		t.Fatalf("первое наблюдение разобрано неверно: %+v", indices)
	}
	validFraction := first.ValidFraction()
	if first.Date().String() != "2025-06-03" || validFraction == nil || *validFraction != 0.95 {
		t.Fatalf("дата или valid_fraction первого наблюдения неверны: %s/%v", first.Date(), first.ValidFraction())
	}
	low := samples[1]
	if low.Usable() || low.Reason() != source.ReasonLowValidFraction {
		t.Fatalf("наблюдение с низким качеством разобрано неверно: %+v", low)
	}
	empty := samples[2]
	emptyIndices := empty.Indices()
	if empty.Usable() || emptyIndices.NDVI != nil || emptyIndices.EVI != nil || emptyIndices.NDWI != nil ||
		empty.Reason() != source.ReasonNoValidSamples {
		t.Fatalf("пустое наблюдение разобрано неверно: %+v", empty)
	}
	mapping := series.Descriptor().Mapping()
	if !strings.Contains(mapping, "EVI=") || !strings.Contains(mapping, "NDWI=") {
		t.Fatalf("описание не содержит формулы индексов: %s", mapping)
	}
}

func TestProviderDescribesProcessing(t *testing.T) {
	service := newService(t)
	series, err := newProvider(t, service, fixedClock()).FetchNDVI(context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-20"))
	if err != nil {
		t.Fatalf("наблюдения не получены: %v", err)
	}
	descriptor := series.Descriptor()
	if descriptor.ID() != cdse.SourceID || descriptor.Kind() != source.KindSatellite {
		t.Fatalf("описание источника %s/%s", descriptor.ID(), descriptor.Kind())
	}
	if descriptor.License() == nil || *descriptor.License() == "" {
		t.Fatal("лицензия источника не заполнена")
	}
	for _, fragment := range []string{"B08", "SCL", "P5D", "valid_fraction"} {
		if !strings.Contains(descriptor.Mapping(), fragment) {
			t.Fatalf("в описании обработки нет %q: %s", fragment, descriptor.Mapping())
		}
	}
	if descriptor.RetrievedAt() != fixedClock()() {
		t.Fatalf("время получения %s", descriptor.RetrievedAt())
	}
}

func TestProviderSendsContractRequest(t *testing.T) {
	service := newService(t)
	if _, err := newProvider(t, service, fixedClock()).FetchNDVI(context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-20")); err != nil {
		t.Fatalf("наблюдения не получены: %v", err)
	}
	if service.lastAuth != "Bearer synthetic-access-token" {
		t.Fatalf("заголовок авторизации %q", service.lastAuth)
	}
	document := map[string]any{}
	if err := json.Unmarshal(service.lastBody, &document); err != nil {
		t.Fatalf("тело запроса не разобрано: %v", err)
	}
	aggregation, ok := document["aggregation"].(map[string]any)
	if !ok {
		t.Fatal("в запросе нет блока aggregation")
	}
	timeRange, ok := aggregation["timeRange"].(map[string]any)
	if !ok || timeRange["to"] != "2025-06-21T00:00:00Z" {
		t.Fatalf("верхняя граница запроса %v, ожидался следующий день после периода", timeRange["to"])
	}
	interval, ok := aggregation["aggregationInterval"].(map[string]any)
	if !ok || interval["of"] != "P5D" {
		t.Fatalf("интервал агрегации %v", interval)
	}
	for _, key := range []string{"resx", "resy"} {
		resolution, ok := aggregation[key].(float64)
		if !ok || resolution <= 0 || resolution >= 0.001 {
			t.Fatalf("%s должен быть положительным разрешением CRS84 в градусах, получено %v", key, aggregation[key])
		}
	}
	resx := aggregation["resx"].(float64)
	resy := aggregation["resy"].(float64)
	latitude := 45.005 * math.Pi / 180
	wantResx := 10 / (111320.0 * math.Abs(math.Cos(latitude)))
	wantResy := 10 / 111320.0
	if math.Abs(resx-wantResx) > 1e-12 || math.Abs(resy-wantResy) > 1e-12 {
		t.Fatalf("разрешение CRS84 %g/%g, ожидалось %g/%g", resx, resy, wantResx, wantResy)
	}
	evalscript, ok := aggregation["evalscript"].(string)
	if !ok || !strings.Contains(evalscript, "dataMask") || !strings.Contains(evalscript, "SCL") {
		t.Fatal("evalscript не содержит маску качества")
	}
	for _, fragment := range []string{"B02", "B11", "evi", "ndwi", "7.5", "Gao"} {
		if !strings.Contains(evalscript, fragment) {
			t.Fatalf("evalscript не содержит расчёт S2 индексов: %q", fragment)
		}
	}
	calculations, ok := document["calculations"].(map[string]any)
	if !ok {
		t.Fatal("в запросе нет блока calculations")
	}
	for _, name := range []string{"ndvi", "evi", "ndwi"} {
		if _, found := calculations[name]; !found {
			t.Fatalf("в calculations нет индекса %s", name)
		}
	}
	if !strings.Contains(string(service.lastBody), "\"type\":\"Polygon\"") {
		t.Fatal("геометрия участка не передана")
	}
}

func TestProviderReusesTokenUntilExpiry(t *testing.T) {
	service := newService(t)
	moment := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return moment }
	provider := newProvider(t, service, clock)

	request := satelliteRequest(t, "2025-06-01", "2025-06-20")
	if _, err := provider.FetchNDVI(context.Background(), request); err != nil {
		t.Fatalf("первый запрос не выполнен: %v", err)
	}
	if _, err := provider.FetchNDVI(context.Background(), request); err != nil {
		t.Fatalf("второй запрос не выполнен: %v", err)
	}
	if service.tokenCalls != 1 {
		t.Fatalf("запросов токена %d, ожидался один", service.tokenCalls)
	}
	moment = moment.Add(11 * time.Minute)
	if _, err := provider.FetchNDVI(context.Background(), request); err != nil {
		t.Fatalf("третий запрос не выполнен: %v", err)
	}
	if service.tokenCalls != 2 {
		t.Fatalf("запросов токена %d, ожидалось обновление после истечения", service.tokenCalls)
	}
}

func TestProviderReportsAuthorizationFailure(t *testing.T) {
	service := newService(t)
	service.tokenStatus = http.StatusUnauthorized
	_, err := newProvider(t, service, fixedClock()).FetchNDVI(context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-20"))
	if domain.KindOf(err) != domain.FailureUnauthorized {
		t.Fatalf("вид ошибки %q", domain.KindOf(err))
	}
	if domain.IsRetryable(err) {
		t.Fatal("ошибка доступа не должна помечаться как повторяемая")
	}
	if service.statisticCall != 0 {
		t.Fatal("запрос статистики выполнен без токена")
	}
}

func TestProviderReportsStatisticsFailure(t *testing.T) {
	service := newService(t)
	service.statusCode = http.StatusServiceUnavailable
	_, err := newProvider(t, service, fixedClock()).FetchNDVI(context.Background(), satelliteRequest(t, "2025-06-01", "2025-06-20"))
	if domain.KindOf(err) != domain.FailureUnavailable {
		t.Fatalf("вид ошибки %q", domain.KindOf(err))
	}
}

func TestCredentialsAreHiddenInLogs(t *testing.T) {
	credentials, err := cdse.NewCredentials("client", "very-secret-value")
	if err != nil {
		t.Fatalf("доступ не построен: %v", err)
	}
	if strings.Contains(credentials.String(), "very-secret-value") {
		t.Fatal("секрет попадает в строковое представление")
	}
	if _, err := cdse.NewCredentials("client", ""); err == nil {
		t.Fatal("пустой секрет принят")
	}
}
