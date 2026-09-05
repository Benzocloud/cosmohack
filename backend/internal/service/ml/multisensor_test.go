package ml

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func multisensorRequest(t *testing.T) *domain.AnalysisRequest {
	t.Helper()
	req := successRequest(t)
	req.SchemaVersion = domain.SchemaVersionV11
	req.FeatureProfile = domain.FeatureProfileMultisensorV1
	ndvi := *req.Observations[0].PrimaryNDVI
	req.Observations[0].Indices = &domain.Indices{S2NDVI: &ndvi}
	return req
}

func TestValidateRequestMultisensor(t *testing.T) {
	t.Parallel()

	valid := multisensorRequest(t)
	tests := []struct {
		name   string
		mutate func(*domain.AnalysisRequest)
		want   domain.MLErrorCode
	}{
		{name: "valid", want: ""},
		{
			name: "primary does not match index",
			mutate: func(req *domain.AnalysisRequest) {
				value := 0.1
				req.Observations[0].Indices.S2NDVI = &value
			},
			want: domain.MLErrorInvalidRequest,
		},
		{
			name: "multisensor requires v1.1",
			mutate: func(req *domain.AnalysisRequest) {
				req.SchemaVersion = domain.SchemaVersionV1
			},
			want: domain.MLErrorContractMismatch,
		},
		{
			name: "duplicate peer",
			mutate: func(req *domain.AnalysisRequest) {
				req.Peers = []domain.PeerSeries{
					{AreaID: "peer-1"},
					{AreaID: "peer-1"},
				}
			},
			want: domain.MLErrorInvalidRequest,
		},
		{
			name: "self peer",
			mutate: func(req *domain.AnalysisRequest) {
				req.Peers = []domain.PeerSeries{{AreaID: req.AreaID}}
			},
			want: domain.MLErrorInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := *valid
			req.Observations = append([]domain.Observation(nil), valid.Observations...)
			if tt.mutate != nil {
				tt.mutate(&req)
			}
			err := validateRequest(&req)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateRequest returned error: %v", err)
				}
				return
			}
			mustMLError(t, err, tt.want)
		})
	}
}

func TestClientAnalyzeMultisensorNegotiation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		readyBody   string
		wantSchema  string
		wantProfile string
	}{
		{
			name:        "advertised extended profile",
			readyBody:   `{"status":"ready","schema_version":"1.0","schema_versions":["1.0","1.1"],"feature_profiles":["ndvi-weather-v1","ndvi-multisensor-v1"],"model_version":"baseline-fixture-1"}`,
			wantSchema:  domain.SchemaVersionV11,
			wantProfile: domain.FeatureProfileMultisensorV1,
		},
		{
			name:        "legacy readiness falls back",
			readyBody:   `{"status":"ready","schema_version":"1.0","feature_profiles":["ndvi-weather-v1"],"model_version":"baseline-fixture-1"}`,
			wantSchema:  domain.SchemaVersionV1,
			wantProfile: domain.FeatureProfileNDVIWeatherV1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got domain.AnalysisRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/readyz" {
					_, _ = io.WriteString(w, tt.readyBody)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read request: %v", err)
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Errorf("decode request: %v", err)
				}
				response := string(fixture(t, "response_success.json"))
				response = strings.Replace(response, `"schema_version": "1.0"`, `"schema_version": "`+got.SchemaVersion+`"`, 1)
				response = strings.Replace(response, `"feature_profile": "ndvi-weather-v1"`, `"feature_profile": "`+got.FeatureProfile+`"`, 1)
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, nil)
			if _, err := client.Analyze(context.Background(), multisensorRequest(t)); err != nil {
				t.Fatalf("Analyze returned error: %v", err)
			}
			if got.SchemaVersion != tt.wantSchema || got.FeatureProfile != tt.wantProfile {
				t.Fatalf("negotiated request = %s/%s, want %s/%s", got.SchemaVersion, got.FeatureProfile, tt.wantSchema, tt.wantProfile)
			}
		})
	}
}
