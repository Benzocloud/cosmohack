package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

const goldenRequestPath = "testdata/ml-http/analyze_request_example.json"

func exampleSnapshot(t *testing.T) *source.Snapshot {
	t.Helper()
	period := mustRange(t, "2025-06-01", "2025-06-10")
	satellite := &stubSatellite{series: satelliteSeries(t,
		sampleFor(t, "2025-06-01", "2025-06-05", source.Float(0.71), source.Float(0.95), true, ""),
		sampleFor(t, "2025-06-06", "2025-06-10", source.Float(0.44), source.Float(0.2), false, source.ReasonLowValidFraction),
	)}
	weather := &stubWeather{series: weatherSeries(t, mustRange(t, "2025-06-01", "2025-06-08"))}
	snapshot, err := newCollector(t, satellite, weather).Collect(context.Background(), collectRequest(t, period))
	if err != nil {
		t.Fatalf("снимок не собран: %v", err)
	}
	return snapshot
}

func TestAnalyzeRequestMatchesContract(t *testing.T) {
	snapshot := exampleSnapshot(t)
	request, err := source.NewAnalyzeRequestBuilder(0, 0).Build(snapshot, "job-b1-example-1")
	if err != nil {
		t.Fatalf("запрос анализа не построен: %v", err)
	}
	document := map[string]any{}
	if err := json.Unmarshal(request.Body(), &document); err != nil {
		t.Fatalf("запрос не разобран: %v", err)
	}
	expected := map[string]any{
		"schema_version":  source.SchemaVersion,
		"request_id":      "job-b1-example-1",
		"area_id":         snapshot.AreaID(),
		"input_revision":  snapshot.Revision(),
		"mode":            source.ModeRetrospective,
		"feature_profile": source.FeatureProfileNDVIWeather,
	}
	for field, want := range expected {
		if document[field] != want {
			t.Fatalf("поле %s равно %v, ожидалось %v", field, document[field], want)
		}
	}
	observations, ok := document["observations"].([]any)
	if !ok || len(observations) != snapshot.Period().Days() {
		t.Fatalf("наблюдений в запросе %d, ожидалось %d", len(observations), snapshot.Period().Days())
	}
	first, ok := observations[0].(map[string]any)
	if !ok {
		t.Fatal("наблюдение не является объектом")
	}
	for _, field := range []string{"date", "primary_ndvi", "quality", "ndvi_source_id", "interval", "valid_fraction", "missing_reason", "weather", "reference"} {
		if _, found := first[field]; !found {
			t.Fatalf("в наблюдении нет обязательного поля %s", field)
		}
	}
	if bytes.Contains(request.Body(), []byte("NaN")) || bytes.Contains(request.Body(), []byte("Inf")) {
		t.Fatal("запрос содержит неконечные значения")
	}
}

func TestAnalyzeRequestMatchesGoldenExample(t *testing.T) {
	request, err := source.NewAnalyzeRequestBuilder(0, 0).Build(exampleSnapshot(t), "job-b1-example-1")
	if err != nil {
		t.Fatalf("запрос анализа не построен: %v", err)
	}
	formatted := bytes.Buffer{}
	if err := json.Indent(&formatted, request.Body(), "", "  "); err != nil {
		t.Fatalf("запрос не отформатирован: %v", err)
	}
	formatted.WriteString("\n")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenRequestPath), 0o755); err != nil {
			t.Fatalf("каталог фикстуры не создан: %v", err)
		}
		if err := os.WriteFile(goldenRequestPath, formatted.Bytes(), 0o644); err != nil {
			t.Fatalf("фикстура не записана: %v", err)
		}
	}
	golden, err := os.ReadFile(goldenRequestPath)
	if err != nil {
		t.Fatalf("фикстура не прочитана: %v", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(golden, []byte("\r\n"), []byte("\n")), formatted.Bytes()) {
		t.Fatal("запрос анализа отличается от согласованного примера; обновление фикстуры выполняется с UPDATE_GOLDEN=1")
	}
}

func TestAnalyzeRequestRejectsInvalidInput(t *testing.T) {
	snapshot := exampleSnapshot(t)
	if _, err := source.NewAnalyzeRequestBuilder(0, 0).Build(snapshot, ""); err == nil {
		t.Fatal("запрос без request_id принят")
	}
	if _, err := source.NewAnalyzeRequestBuilder(0, 0).Build(nil, "job-1"); err == nil {
		t.Fatal("запрос без снимка принят")
	}
	if _, err := source.NewAnalyzeRequestBuilder(3, 0).Build(snapshot, "job-1"); err == nil {
		t.Fatal("превышение числа наблюдений не отклонено")
	}
	if _, err := source.NewAnalyzeRequestBuilder(0, 64).Build(snapshot, "job-1"); err == nil {
		t.Fatal("превышение размера тела не отклонено")
	}
}

func TestAnalyzeRequestKeepsObservationOrder(t *testing.T) {
	request, err := source.NewAnalyzeRequestBuilder(0, 0).Build(exampleSnapshot(t), "job-b1-example-1")
	if err != nil {
		t.Fatalf("запрос анализа не построен: %v", err)
	}
	document := struct {
		Observations []struct {
			Date source.Date `json:"date"`
		} `json:"observations"`
	}{}
	if err := json.Unmarshal(request.Body(), &document); err != nil {
		t.Fatalf("запрос не разобран: %v", err)
	}
	for index := 1; index < len(document.Observations); index++ {
		if !document.Observations[index-1].Date.Before(document.Observations[index].Date) {
			t.Fatalf("даты не строго возрастают на позиции %d", index)
		}
	}
}
