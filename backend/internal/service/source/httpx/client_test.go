package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/httpx"
)

type payload struct {
	Value string `json:"value"`
}

func newRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("запрос не построен: %v", err)
	}
	return request
}

func TestClientDecodesJSONAndSendsUserAgent(t *testing.T) {
	agent := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		agent <- request.Header.Get("User-Agent")
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	client := httpx.NewClient(httpx.WithUserAgent("cosmohack-b1-test"))
	result := &payload{}
	if err := client.DoJSON(context.Background(), "test", newRequest(t, server.URL), result); err != nil {
		t.Fatalf("запрос не выполнен: %v", err)
	}
	if result.Value != "ok" {
		t.Fatalf("значение ответа %q", result.Value)
	}
	if sent := <-agent; sent != "cosmohack-b1-test" {
		t.Fatalf("User-Agent %q", sent)
	}
}

func TestClientMapsStatusToFailureKind(t *testing.T) {
	cases := map[int]source.FailureKind{
		http.StatusUnauthorized:       source.FailureUnauthorized,
		http.StatusForbidden:          source.FailureUnauthorized,
		http.StatusTooManyRequests:    source.FailureRateLimited,
		http.StatusGatewayTimeout:     source.FailureTimeout,
		http.StatusServiceUnavailable: source.FailureUnavailable,
		http.StatusBadRequest:         source.FailureInvalidRequest,
	}
	for status, expected := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		err := httpx.NewClient().DoJSON(context.Background(), "test", newRequest(t, server.URL), &payload{})
		server.Close()
		if source.KindOf(err) != expected {
			t.Fatalf("статус %d дал вид %q, ожидался %q", status, source.KindOf(err), expected)
		}
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"value":"` + strings.Repeat("x", 4096) + `"}`))
	}))
	defer server.Close()

	client := httpx.NewClient(httpx.WithMaxResponseBytes(128))
	err := client.DoJSON(context.Background(), "test", newRequest(t, server.URL), &payload{})
	if source.KindOf(err) != source.FailureMalformed {
		t.Fatalf("вид ошибки %q, ожидался %q", source.KindOf(err), source.FailureMalformed)
	}
}

func TestClientRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"value":`))
	}))
	defer server.Close()

	err := httpx.NewClient().DoJSON(context.Background(), "test", newRequest(t, server.URL), &payload{})
	if source.KindOf(err) != source.FailureMalformed {
		t.Fatalf("вид ошибки %q", source.KindOf(err))
	}
}

func TestClientReportsTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	client := httpx.NewClient(httpx.WithTimeout(50 * time.Millisecond))
	err := client.DoJSON(context.Background(), "test", newRequest(t, server.URL), &payload{})
	if source.KindOf(err) != source.FailureTimeout {
		t.Fatalf("вид ошибки %q, ожидался %q", source.KindOf(err), source.FailureTimeout)
	}
	if !source.IsRetryable(err) {
		t.Fatal("тайм-аут должен допускать явный повтор")
	}
}
