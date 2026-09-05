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
