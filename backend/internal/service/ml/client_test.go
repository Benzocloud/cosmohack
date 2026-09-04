package ml

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// fixture загружает общий пример HTTP-контракта из backend/testdata/ml-http.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "ml-http", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// mustMLError проверяет, что ошибка — *ml.Error с ожидаемым кодом контракта.
func mustMLError(t *testing.T, err error, want domain.MLErrorCode) *Error {
	t.Helper()
	var mlErr *Error
	if !errors.As(err, &mlErr) || mlErr.Code != want {
		t.Fatalf("want %s, got %v", want, err)
	}
	return mlErr
}

func newTestClient(t *testing.T, serverURL string, mutate func(*Config)) *Client {
	t.Helper()
	cfg := DefaultConfig(serverURL)
	cfg.ExpectedModelVersion = "baseline-fixture-1"
	if mutate != nil {
		mutate(&cfg)
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

// successRequest разбирает фикстуру успешного запроса в доменную структуру.
func successRequest(t *testing.T) *domain.AnalysisRequest {
	t.Helper()
	var req domain.AnalysisRequest
	if err := json.Unmarshal(fixture(t, "request_success.json"), &req); err != nil {
		t.Fatalf("decode request fixture: %v", err)
	}
	return &req
}

func TestClient_Analyze_Success(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/analyze" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q, want application/json", got)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "response_success.json"))
	}))
	defer server.Close()

	req := successRequest(t)
	client := newTestClient(t, server.URL, nil)
	result, err := client.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	checkSuccessResult(t, result)

	var sent domain.AnalysisRequest
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("sent body is not valid analyze json: %v", err)
	}
	if sent.RequestID != req.RequestID || sent.AreaID != req.AreaID {
		t.Fatalf("sent body lost correlation fields: %s/%s", sent.RequestID, sent.AreaID)
	}
}

// checkSuccessResult фиксирует содержательные поля успешного результата.
func checkSuccessResult(t *testing.T, result *domain.AnalysisResult) {
	t.Helper()
	if result.Status != domain.StatusCandidate || result.Severity == nil || *result.Severity != domain.SeverityModerate {
		t.Fatalf("unexpected result status/severity: %s/%v", result.Status, result.Severity)
	}
	if len(result.Series) != 3 {
		t.Fatalf("series length = %d, want 3", len(result.Series))
	}
	if result.Series[1].State != domain.StateImputed || result.Series[1].Value == nil {
		t.Fatalf("series[1] = %+v, want imputed with value", result.Series[1])
	}
	if len(result.Events) != 1 || result.Events[0].Status != domain.StatusCandidate {
		t.Fatalf("unexpected events: %+v", result.Events)
	}
}

func TestClient_Analyze_InsufficientData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "response_insufficient.json"))
	}))
	defer server.Close()

	var req domain.AnalysisRequest
	if err := json.Unmarshal(fixture(t, "request_insufficient.json"), &req); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	client := newTestClient(t, server.URL, func(c *Config) { c.ExpectedModelVersion = "baseline-example-1" })
	result, err := client.Analyze(context.Background(), &req)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.Status != domain.StatusInsufficientData || result.Severity != nil {
		t.Fatalf("unexpected status/severity: %s/%v", result.Status, result.Severity)
	}
}

func TestClient_Analyze_ErrorMapping(t *testing.T) {
	t.Parallel()

	// Таблица повторяет таблицу обработки ошибок HTTP-контракта.
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode domain.MLErrorCode
	}{
		{"invalid json", http.StatusBadRequest, `{"schema_version":"1.0","request_id":"job-fixture-1","error":{"code":"invalid_json","message":"bad json","retryable":false}}`, domain.MLErrorInvalidRequest},
		{"payload too large", http.StatusRequestEntityTooLarge, `{"schema_version":"1.0","request_id":"job-fixture-1","error":{"code":"payload_too_large","message":"too large","retryable":false}}`, domain.MLErrorInputTooLarge},
		{"unsupported media type", http.StatusUnsupportedMediaType, `{"schema_version":"1.0","request_id":"job-fixture-1","error":{"code":"unsupported_media_type","message":"media type","retryable":false}}`, domain.MLErrorInvalidRequest},
		{"invalid input", http.StatusUnprocessableEntity, string(fixture(t, "error_invalid_input.json")), domain.MLErrorInvalidRequest},
		{"unsupported contract", http.StatusUnprocessableEntity, string(fixture(t, "error_unsupported_contract.json")), domain.MLErrorContractMismatch},
		{"busy", http.StatusTooManyRequests, string(fixture(t, "error_busy.json")), domain.MLErrorBusy},
		{"not ready", http.StatusServiceUnavailable, `{"schema_version":"1.0","request_id":"job-fixture-1","error":{"code":"not_ready","message":"not ready","retryable":true}}`, domain.MLErrorUnavailable},
		{"internal error", http.StatusInternalServerError, `{"schema_version":"1.0","request_id":"job-fixture-1","error":{"code":"internal_error","message":"boom","retryable":true}}`, domain.MLErrorInternal},
		{"foreign request id in error", http.StatusTooManyRequests, `{"schema_version":"1.0","request_id":"job-other","error":{"code":"busy","message":"busy","retryable":true}}`, domain.MLErrorInvalidResponse},
		{"foreign schema version in error", http.StatusTooManyRequests, `{"schema_version":"2.0","request_id":"job-fixture-1","error":{"code":"busy","message":"busy","retryable":true}}`, domain.MLErrorInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, nil)
			_, err := client.Analyze(context.Background(), successRequest(t))
			mlErr := mustMLError(t, err, tt.wantCode)
			if mlErr.Message == "" {
				t.Fatal("error message must not be empty")
			}
		})
	}
}

func TestClient_Analyze_BusyKeepsMLMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write(fixture(t, "error_busy.json"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil)
	_, err := client.Analyze(context.Background(), successRequest(t))
	mlErr := mustMLError(t, err, domain.MLErrorBusy)
	if !mlErr.Retryable {
		t.Fatal("busy error must keep retryable=true")
	}
}

func TestClient_Analyze_InvalidResponse(t *testing.T) {
	t.Parallel()

	// jsonMutation правит структуру фикстуры и пере-кодирует её.
	jsonMutation := func(apply func(map[string]any)) func([]byte) []byte {
		return func(body []byte) []byte {
			var obj map[string]any
			if err := json.Unmarshal(body, &obj); err != nil {
				t.Fatalf("fixture is not a json object: %v", err)
			}
			apply(obj)
			out, err := json.Marshal(obj)
			if err != nil {
				t.Fatalf("re-marshal mutated fixture: %v", err)
			}
			return out
		}
	}
	firstEvent := func(m map[string]any) map[string]any {
		return m["events"].([]any)[0].(map[string]any)
	}

	tests := []struct {
		name   string
		mutate func(body []byte) []byte
	}{
		{"not json", func([]byte) []byte { return []byte("not json") }},
		{"wrong schema version", replaceInBody(`"schema_version": "1.0"`, `"schema_version": "2.0"`)},
		{"foreign request id", replaceInBody(`"request_id": "job-fixture-1"`, `"request_id": "job-other"`)},
		{"wrong area id", replaceInBody(`"area_id": "area-fixture-1"`, `"area_id": "area-other"`)},
		{"wrong input revision", replaceInBody(`"input_revision": "input-fixture-1"`, `"input_revision": "input-other"`)},
		{"empty model version", replaceInBody(`"model_version": "baseline-fixture-1"`, `"model_version": ""`)},
		{"no method", replaceInBody(`"method": "hist_gradient_boosting_v1",`, `"method": "",`)},
		{"unknown status", replaceInBody(`"status": "candidate"`, `"status": "unknown"`)},
		{"normal with wrong severity", replaceInBody("\"status\": \"candidate\",\n  \"severity\": \"moderate\"", "\"status\": \"normal\",\n  \"severity\": \"moderate\"")},
		{"missing series date", replaceInBody(`{"date": "2026-06-05", "primary_ndvi": null, "value": 0.44, "state": "imputed", "method": "hist_gradient_boosting_v1", "baseline": 0.6, "z_score": -1.6},`, ``)},
		{"changed original ndvi", replaceInBody(`"primary_ndvi": 0.31, "value": 0.31, "state": "observed"`, `"primary_ndvi": 0.9, "value": 0.9, "state": "observed"`)},
		{"observed value mismatch", replaceInBody(`"primary_ndvi": 0.58, "value": 0.58, "state": "observed"`, `"primary_ndvi": 0.58, "value": 0.59, "state": "observed"`)},
		{"imputed without method", replaceInBody(`"state": "imputed", "method": "hist_gradient_boosting_v1"`, `"state": "imputed", "method": null`)},
		{"missing with value", replaceInBody(`"primary_ndvi": null, "value": 0.44, "state": "imputed"`, `"primary_ndvi": null, "value": null, "state": "missing"`)},
		{"usable input returned as imputed", jsonMutation(func(m map[string]any) {
			point := m["series"].([]any)[0].(map[string]any)
			point["state"] = "imputed"
			point["method"] = "hist_gradient_boosting_v1"
		})},
		{"unknown event status", replaceInBody("\"status\": \"candidate\",\n      \"severity\": \"moderate\"", "\"status\": \"unknown\",\n      \"severity\": \"moderate\"")},
		{"event start after end", jsonMutation(func(m map[string]any) {
			ev := firstEvent(m)
			ev["start_date"] = "2026-07-10"
			ev["end_date"] = "2026-07-01"
		})},
		{"event outside period", jsonMutation(func(m map[string]any) {
			firstEvent(m)["end_date"] = "2026-08-01"
		})},
		{"null events", jsonMutation(func(m map[string]any) { m["events"] = nil })},
		{"candidate without events", jsonMutation(func(m map[string]any) { m["events"] = []any{} })},
		{"normal with events", jsonMutation(func(m map[string]any) {
			m["status"] = "normal"
			m["severity"] = "none"
		})},
		{"insufficient with events", jsonMutation(func(m map[string]any) {
			m["status"] = "insufficient_data"
			m["severity"] = nil
		})},
		{"confirmed without confirmed event", jsonMutation(func(m map[string]any) {
			m["status"] = "confirmed"
			m["severity"] = "high"
		})},
		{"null evidence", jsonMutation(func(m map[string]any) { firstEvent(m)["evidence_dates"] = nil })},
		{"evidence not observed", replaceInBody(`"evidence_dates": ["2026-07-10"]`, `"evidence_dates": ["2026-06-05"]`)},
		{"null facts", jsonMutation(func(m map[string]any) { firstEvent(m)["facts"] = nil })},
		{"candidate with confirmed event", jsonMutation(func(m map[string]any) {
			firstEvent(m)["status"] = "confirmed"
		})},
		{"severity below events", jsonMutation(func(m map[string]any) {
			firstEvent(m)["severity"] = "high"
		})},
		{"null series", jsonMutation(func(m map[string]any) { m["series"] = nil })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write(tt.mutate(fixture(t, "response_success.json")))
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, nil)
			_, err := client.Analyze(context.Background(), successRequest(t))
			mustMLError(t, err, domain.MLErrorInvalidResponse)
		})
	}
}

func TestClient_Analyze_UnexpectedModelVersion(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "response_success.json"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, func(c *Config) { c.ExpectedModelVersion = "other-model" })
	_, err := client.Analyze(context.Background(), successRequest(t))
	mustMLError(t, err, domain.MLErrorInvalidResponse)
}

func TestClient_Analyze_ResponseLimitAndContentType(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		handler http.HandlerFunc
	}{
		{
			name:   "body exceeds limit",
			mutate: func(c *Config) { c.MaxResponseBodyBytes = 8 },
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write(fixture(t, "response_success.json"))
			},
		},
		{
			name: "non-json content type",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("{}"))
			},
		},
		{
			name: "unknown http status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("gateway"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := newTestClient(t, server.URL, tt.mutate)
			_, err := client.Analyze(context.Background(), successRequest(t))
			mustMLError(t, err, domain.MLErrorInvalidResponse)
		})
	}
}

func TestClient_Analyze_TransportFailures(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		server.Close()

		client := newTestClient(t, server.URL, nil)
		_, err := client.Analyze(context.Background(), successRequest(t))
		mustMLError(t, err, domain.MLErrorUnavailable)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, func(c *Config) { c.AnalyzeTimeout = 50 * time.Millisecond })
		_, err := client.Analyze(context.Background(), successRequest(t))
		mustMLError(t, err, domain.MLErrorTimeout)
	})

	t.Run("context canceled passes through", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		client := newTestClient(t, server.URL, nil)
		cancel()
		_, err := client.Analyze(ctx, successRequest(t))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	})
}

func TestClient_Analyze_RejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*domain.AnalysisRequest)
	}{
		{"empty request id", func(r *domain.AnalysisRequest) { r.RequestID = "" }},
		{"too long area id", func(r *domain.AnalysisRequest) { r.AreaID = strings.Repeat("a", domain.MaxIDLength+1) }},
		{"bad mode", func(r *domain.AnalysisRequest) { r.Mode = "forecast" }},
		{"bad profile", func(r *domain.AnalysisRequest) { r.FeatureProfile = "ndvi-only-v2" }},
		{"period inverted", func(r *domain.AnalysisRequest) {
			r.AnalysisPeriod = domain.Period{From: "2026-08-01", To: "2026-05-01"}
		}},
		{"duplicate source id", func(r *domain.AnalysisRequest) {
			r.Sources = append(r.Sources, r.Sources[0])
		}},
		{"retrieved at not utc", func(r *domain.AnalysisRequest) {
			r.Sources[0].RetrievedAt = "2026-08-01T12:00:00+02:00"
		}},
		{"duplicate observation date", func(r *domain.AnalysisRequest) {
			r.Observations[2].Date = r.Observations[1].Date
		}},
		{"dates not ascending", func(r *domain.AnalysisRequest) {
			r.Observations[1].Date, r.Observations[2].Date = r.Observations[2].Date, r.Observations[1].Date
		}},
		{"usable without ndvi", func(r *domain.AnalysisRequest) {
			r.Observations[1].PrimaryNDVI = nil
		}},
		{"usable without interval", func(r *domain.AnalysisRequest) {
			r.Observations[1].Interval = nil
		}},
		{"unusable without reason", func(r *domain.AnalysisRequest) {
			r.Observations[1].Quality = domain.QualityUnusable
			r.Observations[1].MissingReason = nil
		}},
		{"missing without reason", func(r *domain.AnalysisRequest) {
			r.Observations[2].MissingReason = nil
		}},
		{"missing with value", func(r *domain.AnalysisRequest) {
			v := 0.5
			r.Observations[2].PrimaryNDVI = &v
		}},
		{"weather references satellite source", func(r *domain.AnalysisRequest) {
			r.Observations[1].Weather.SourceID = "src-sentinel-1"
		}},
		{"negative precipitation", func(r *domain.AnalysisRequest) {
			v := -0.1
			r.Observations[1].Weather.PrecipitationSumMM = &v
		}},
		{"reference without excluded year", func(r *domain.AnalysisRequest) {
			r.Observations[1].Reference.TargetYearExcluded = false
		}},
		{"bad valid fraction", func(r *domain.AnalysisRequest) {
			v := 1.5
			r.Observations[1].ValidFraction = &v
		}},
		{"too many observations", func(r *domain.AnalysisRequest) {
			reason := "no_usable_observation"
			for i := range domain.MaxObservationsPerRequest {
				obs := domain.Observation{Quality: domain.QualityMissing, MissingReason: &reason}
				obs.Date = nextFixtureDate(i)
				r.Observations = append(r.Observations, obs)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			req := successRequest(t)
			tt.mutate(req)
			client := newTestClient(t, server.URL, nil)
			_, err := client.Analyze(context.Background(), req)
			mustMLError(t, err, domain.MLErrorInvalidRequest)
			if called {
				t.Fatal("invalid request must not reach the ml server")
			}
		})
	}
}

// nextFixtureDate строит возрастающие даты после последней точки фикстуры,
// чтобы кейс «слишком много наблюдений» не ломал порядок дат.
func nextFixtureDate(i int) string {
	base := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, i).Format("2006-01-02")
}

func TestClient_Analyze_RequestTooLarge(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("oversized request must not reach the ml server")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, func(c *Config) { c.MaxRequestBodyBytes = 16 })
	_, err := client.Analyze(context.Background(), successRequest(t))
	mustMLError(t, err, domain.MLErrorInputTooLarge)
}

func TestClient_Ready(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/readyz" {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(fixture(t, "readyz_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		info, err := client.Ready(context.Background())
		if err != nil {
			t.Fatalf("Ready returned error: %v", err)
		}
		if info.Status != domain.MLReadyStatus || info.ModelVersion != "baseline-fixture-1" {
			t.Fatalf("unexpected ready info: %+v", info)
		}
	})

	t.Run("not ready 503", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write(fixture(t, "readyz_not_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		info, err := client.Ready(context.Background())
		if err != nil {
			t.Fatalf("503 readiness must not be a transport error: %v", err)
		}
		if info.Status != "not_ready" {
			t.Fatalf("status = %q, want not_ready", info.Status)
		}
	})

	t.Run("ready 503 must not claim ready", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write(fixture(t, "readyz_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		_, err := client.Ready(context.Background())
		mustMLError(t, err, domain.MLErrorInvalidResponse)
	})

	t.Run("ready with charset parameter", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(fixture(t, "readyz_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		if _, err := client.Ready(context.Background()); err != nil {
			t.Fatalf("charset parameter must be accepted: %v", err)
		}
	})

	t.Run("non-json content type", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write(fixture(t, "readyz_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		_, err := client.Ready(context.Background())
		mustMLError(t, err, domain.MLErrorInvalidResponse)
	})

	t.Run("503 without reason", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		_, err := client.Ready(context.Background())
		mustMLError(t, err, domain.MLErrorInvalidResponse)
	})

	t.Run("unexpected model version", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(fixture(t, "readyz_ready.json"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, func(c *Config) { c.ExpectedModelVersion = "other-model" })
		if _, err := client.Ready(context.Background()); err == nil {
			t.Fatal("expected error for unexpected model version")
		}
	})

	t.Run("malformed body", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("oops"))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, nil)
		_, err := client.Ready(context.Background())
		mustMLError(t, err, domain.MLErrorInvalidResponse)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, func(c *Config) { c.ReadyTimeout = 50 * time.Millisecond })
		_, err := client.Ready(context.Background())
		mustMLError(t, err, domain.MLErrorTimeout)
	})
}

// replaceInBody возвращает мутатор фикстуры ответа простой заменой подстроки.
func replaceInBody(old, new string) func([]byte) []byte {
	return func(body []byte) []byte {
		replaced := strings.ReplaceAll(string(body), old, new)
		return []byte(replaced)
	}
}
