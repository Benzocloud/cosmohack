package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func TestSeriesMixedStates(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobRunning(body.JobID, "analyze"); err != nil {
		t.Fatal(err)
	}
	a, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Series  []domain.SeriesPoint  `json:"series"`
		Weather []domain.WeatherPoint `json:"weather"`
	}
	if err := json.Unmarshal(testdata(t, "series-mixed-states.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	res := domain.AnalysisRecord{
		ResultVersion:  newTestID(),
		AreaID:         id,
		Period:         domain.Period{From: "2024-06-01", To: "2024-06-03"},
		ComputedAt:     time.Now().UTC(),
		SchemaVersion:  "1.0",
		FeatureProfile: "ndvi-weather-v1",
		ModelVersion:   "baseline-example-1",
		Method:         "nearest_mean",
		Status:         "insufficient_data",
		Series:         fixture.Series,
		Weather:        fixture.Weather[:1],
		Limitations:    []string{"Нет проверенного сезонного фона"},
		Events:         []domain.AnomalyEvent{},
	}
	if err := st.PutResult(id, a.Generation, body.JobID, res); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/series", nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var series struct {
		Series []struct {
			State string `json:"state"`
		} `json:"series"`
		Weather []struct {
			Date string `json:"date"`
		} `json:"weather"`
	}
	decode(t, w, &series)
	if len(series.Series) != 3 || series.Series[0].State != "observed" || series.Series[1].State != "imputed" || series.Series[2].State != "missing" {
		t.Fatalf("%s", w.Body.String())
	}
	if len(series.Weather) != 3 {
		t.Fatalf("weather n=%d", len(series.Weather))
	}
	var raw map[string]any
	decode(t, w, &raw)
	weather := raw["weather"].([]any)
	d2 := weather[1].(map[string]any)
	if d2["date"] != "2024-06-02" || d2["temperature_mean_c"] != nil || d2["precipitation_sum_mm"] != nil {
		t.Fatalf("missing weather must be null: %v", d2)
	}
}

func TestJobNotFound(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	w := doJSON(t, h, http.MethodGet, "/api/jobs/no-such-job", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestCompletedResultVisibleAfterHandlerRestart(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start analysis: %d %s", w.Code, w.Body.String())
	}
	var started struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &started)
	if err := st.SetJobRunning(started.JobID, "analyze"); err != nil {
		t.Fatal(err)
	}
	area, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	version := "result-" + newTestID()
	result := domain.AnalysisRecord{
		AreaID:         id,
		ResultVersion:  version,
		Period:         area.Period,
		ComputedAt:     time.Now().UTC(),
		SchemaVersion:  "1.0",
		FeatureProfile: "ndvi-weather-v1",
		ModelVersion:   "test-model",
		Method:         "nearest_mean",
		Status:         domain.StatusNormal,
		Series:         []domain.SeriesPoint{},
		Weather:        []domain.WeatherPoint{},
		Provenance:     map[string]any{"source": "test"},
		Limitations:    []string{},
		Events:         []domain.AnomalyEvent{},
	}
	if err := st.PutResult(id, area.Generation, started.JobID, result); err != nil {
		t.Fatal(err)
	}

	// Пересобирает обработчик над тем же хранилищем, как после перезапуска приложения.
	h = newEnvWithStore(t, st, nil, nil)
	paths := []string{
		"/api/areas/" + id,
		"/api/areas",
		"/api/areas/" + id + "/series",
		"/api/areas/" + id + "/events",
	}
	for _, path := range paths {
		w = doJSON(t, h, http.MethodGet, path, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
		}
		var payload any
		decode(t, w, &payload)
		switch path {
		case "/api/areas/" + id:
			areaPayload := payload.(map[string]any)
			shown := areaPayload["shown_result"].(map[string]any)
			if shown["result_version"] != version {
				t.Fatalf("area shown result=%v", shown)
			}
		case "/api/areas":
			list := payload.(map[string]any)
			areas := list["areas"].([]any)
			shown := areas[0].(map[string]any)["shown_result"].(map[string]any)
			if shown["result_version"] != version {
				t.Fatalf("list shown result=%v", shown)
			}
		case "/api/areas/" + id + "/series":
			if payload.(map[string]any)["result_version"] != version {
				t.Fatalf("series result=%v", payload)
			}
		case "/api/areas/" + id + "/events":
			if payload.(map[string]any)["result_version"] != version {
				t.Fatalf("events result=%v", payload)
			}
		}
	}
}
