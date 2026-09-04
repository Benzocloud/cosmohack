package cdse_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/integration/cdse"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

type stubService struct {
	server        *httptest.Server
	tokenCalls    int32
	statisticCall int32
	lastBody      []byte
	lastAuth      string
	tokenStatus   int
	statusCode    int
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
		writer.Write(fixture(t, "statistics_synthetic.json"))
	})
	service.server = httptest.NewServer(mux)
	t.Cleanup(service.server.Close)
	return service
}

func newProvider(t *testing.T, service *stubService, clock source.Clock) *cdse.Provider {
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

func fixedClock() source.Clock {
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
	evalscript, ok := aggregation["evalscript"].(string)
	if !ok || !strings.Contains(evalscript, "dataMask") || !strings.Contains(evalscript, "SCL") {
		t.Fatal("evalscript не содержит маску качества")
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
	if source.KindOf(err) != source.FailureUnauthorized {
		t.Fatalf("вид ошибки %q", source.KindOf(err))
	}
	if source.IsRetryable(err) {
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
	if source.KindOf(err) != source.FailureUnavailable {
		t.Fatalf("вид ошибки %q", source.KindOf(err))
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
