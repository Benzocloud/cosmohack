package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegister_Ready(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Register(mux, nil)

	getReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", getRec.Code, http.StatusOK)
	}
	if got := getRec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	var body readyResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readyz body: %v", err)
	}
	if body.Status != "ready" || body.SchemaVersion != "1.0" {
		t.Fatalf("unexpected readyz body: %+v", body)
	}
	if len(body.FeatureProfiles) != 1 || body.FeatureProfiles[0] != "ndvi-weather-v1" {
		t.Fatalf("unexpected feature profiles: %v", body.FeatureProfiles)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/readyz", nil)
	postRec := httptest.NewRecorder()
	mux.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", postRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRegister_ReadyDependencyFailure(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, func(context.Context) error { return errors.New("database unavailable") })

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
