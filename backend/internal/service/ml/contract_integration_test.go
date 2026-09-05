//go:build integration

package ml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

const (
	contractMLURLVariable   = "ML_CONTRACT_URL"
	contractMLModelVariable = "ML_CONTRACT_MODEL_VERSION"
)

func TestContract_AnalyzeAgainstFastAPI(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv(contractMLURLVariable))
	if baseURL == "" {
		t.Skipf("set %s to run the live Go-to-FastAPI contract test", contractMLURLVariable)
	}
	expectedModelVersion := strings.TrimSpace(os.Getenv(contractMLModelVariable))
	if expectedModelVersion == "" {
		t.Skipf("set %s to validate the release model version", contractMLModelVariable)
	}

	var request domain.AnalysisRequest
	if err := json.Unmarshal(fixture(t, "analyze_request_example.json"), &request); err != nil {
		t.Fatalf("decode canonical analyze request fixture: %v", err)
	}

	cfg := DefaultConfig(baseURL)
	cfg.ExpectedModelVersion = expectedModelVersion
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("build ML client for %s: %v", baseURL, err)
	}

	result, err := client.Analyze(t.Context(), &request)
	if err != nil {
		t.Fatalf("analyze canonical fixture against ML at %s: %v", baseURL, err)
	}
	if result == nil {
		t.Fatal("analyze returned a nil result")
	}
	t.Logf("ML contract accepted: model_version=%s status=%s series=%d events=%d",
		result.ModelVersion, result.Status, len(result.Series), len(result.Events))
}
