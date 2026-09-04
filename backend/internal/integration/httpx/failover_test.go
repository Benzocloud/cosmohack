package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/httpx"
)

func factory() httpx.RequestFactory {
	return func(ctx context.Context, endpoint string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	}
}

func newFailover(t *testing.T, primary, fallback string) *httpx.Failover {
	t.Helper()
	endpoints, err := httpx.NewEndpoints(primary, fallback)
	if err != nil {
		t.Fatalf("адреса не приняты: %v", err)
	}
	failover, err := httpx.NewFailover(httpx.NewClient(), endpoints)
	if err != nil {
		t.Fatalf("резервный вызов не построен: %v", err)
	}
	return failover
}

func TestFailoverUsesFallbackAfterRetryableFailure(t *testing.T) {
	var primaryCalls, fallbackCalls int32
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&primaryCalls, 1)
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&fallbackCalls, 1)
		writer.Write([]byte(`{"value":"fallback"}`))
	}))
	defer fallback.Close()

	result := &payload{}
	used, err := newFailover(t, primary.URL, fallback.URL).DoJSON(context.Background(), "test", factory(), result)
	if err != nil {
		t.Fatalf("резервный адрес не сработал: %v", err)
	}
	if used != fallback.URL || result.Value != "fallback" {
		t.Fatalf("использован адрес %s со значением %q", used, result.Value)
	}
	if primaryCalls != 1 || fallbackCalls != 1 {
		t.Fatalf("вызовов основного %d, резервного %d", primaryCalls, fallbackCalls)
	}
}

func TestFailoverKeepsNonRetryableFailure(t *testing.T) {
	var fallbackCalls int32
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&fallbackCalls, 1)
		writer.Write([]byte(`{"value":"fallback"}`))
	}))
	defer fallback.Close()

	_, err := newFailover(t, primary.URL, fallback.URL).DoJSON(context.Background(), "test", factory(), &payload{})
	if domain.KindOf(err) != domain.FailureInvalidRequest {
		t.Fatalf("вид ошибки %q", domain.KindOf(err))
	}
	if fallbackCalls != 0 {
		t.Fatal("резервный адрес вызван при неисправимой ошибке основного")
	}
}

func TestFailoverReportsBothFailures(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer fallback.Close()

	_, err := newFailover(t, primary.URL, fallback.URL).DoJSON(context.Background(), "test", factory(), &payload{})
	if err == nil {
		t.Fatal("отказ обоих адресов не сообщён")
	}
	if domain.KindOf(err) != domain.FailureRateLimited {
		t.Fatalf("вид ошибки %q, ожидался вид отказа резервного адреса", domain.KindOf(err))
	}
}

func TestEndpointsValidateURLs(t *testing.T) {
	if _, err := httpx.NewEndpoints("ftp://example.org", ""); err == nil {
		t.Fatal("адрес с неподдерживаемой схемой принят")
	}
	if _, err := httpx.NewEndpoints("https://example.org", "not-a-url"); err == nil {
		t.Fatal("некорректный резервный адрес принят")
	}
	endpoints, err := httpx.NewEndpoints("https://example.org", "")
	if err != nil {
		t.Fatalf("адрес не принят: %v", err)
	}
	if endpoints.HasFallback() {
		t.Fatal("резервный адрес объявлен без значения")
	}
}
